#!/usr/bin/env bash
# Update and rollback on an installed nOS, through the web API, as the
# Updates page does it. tests/smoke/iso-install-test.sh runs it (with
# UPGRADE_REPO) once the installed system is up: the system's APT sources
# then include a repository with a newer nosd (next-version-repo.sh).
#
#   tests/upgrade/upgrade-test.sh <base URL> <setup token> <new nosd version>
#
# Checks: the update check offers the new nosd; applying it saves /etc/nos
# first and installs it; a rollback restores /etc/nos as it was before the
# update (a share made after the update is gone, one made before is kept)
# and nosd comes back.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${1:?base URL, e.g. https://127.0.0.1:18443}"
TOKEN="${2:?setup token from /api/v1/setup/otp/verify}"
NEXT="${3:?the newer nosd version}"
USER_NAME=admin
USER_PASS='Upgrade-Test-Passw0rd!'
WAIT="${UPGRADE_WAIT:-900}"

log() { echo "[upgrade-test] $*"; }
fail() {
	log "FAIL: $*"
	exit 1
}
k() { curl -sk -m 30 "$@"; }

log "finishing first-boot setup as $USER_NAME"
k -fS -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
	-d "$(jq -nc --arg t "$TOKEN" --arg u "$USER_NAME" --arg p "$USER_PASS" '{token:$t,username:$u,password:$p}')" \
	"$BASE/api/v1/setup/first-admin" >/dev/null || fail "first-admin"
k -fS -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d "{\"token\":\"$TOKEN\"}" \
	"$BASE/api/v1/setup/complete" >/dev/null || fail "setup complete"

SESSION_INSECURE=1
# shellcheck source=../../scripts/live-ui/session.sh
. "$ROOT/scripts/live-ui/session.sh" || fail "sign-in"

share_names() { api GET /api/v1/shares | jq -r '.[].name' | sort | paste -sd ' ' -; }
make_share() { api POST /api/v1/shares "$(jq -nc --arg n "$1" --arg u "$USER_NAME" '{name:$n, owners:["user:"+$u]}')" >/dev/null; }

# wait_progress <state...>: until /updates/progress reaches one of the
# states. nosd restarts during the update (its package is upgraded) and
# after a rollback, so failed requests are retried.
wait_progress() {
	local p=""
	for _ in $(seq 1 $((WAIT / 5))); do
		p="$(api GET /api/v1/updates/progress 2>/dev/null || true)"
		for want in "$@"; do
			[ "$(jq -r '.state // empty' <<<"$p" 2>/dev/null)" = "$want" ] && { printf '%s' "$p"; return 0; }
		done
		sleep 5
	done
	fail "update progress did not reach $*: $(jq -c '{state,phase,message,error}' <<<"$p" 2>/dev/null || echo "$p")"
}

make_share before-update || fail "share before the update"

log "checking for updates"
check="$(api POST /api/v1/updates/check)" || fail "update check"
offered="$(jq -r '.latest_version.packages[]? | select(.name == "nosd") | .new_version' <<<"$check")"
[ "$offered" = "$NEXT" ] || fail "the check offers nosd '${offered:-nothing}', want $NEXT: $(jq -c . <<<"$check")"
log "ok: nosd $NEXT is offered"

log "applying the update"
api POST /api/v1/updates/apply '{"packages":["nosd"]}' >/dev/null || fail "update apply"
progress="$(wait_progress success failed)"
[ "$(jq -r .state <<<"$progress")" = success ] || fail "update: $(jq -c '{message,error,logs:[.logs[-8:][]?.message]}' <<<"$progress")"
snapshot="$(jq -r '.snapshot_id // empty' <<<"$progress")"
[ -n "$snapshot" ] || fail "the update saved no snapshot"
log "ok: updated (snapshot $snapshot)"

check="$(api POST /api/v1/updates/check)" || fail "update check after the update"
[ -z "$(jq -r '.latest_version.packages[]? | select(.name == "nosd") | .new_version' <<<"$check")" ] ||
	fail "nosd is still offered after the update: $(jq -c . <<<"$check")"
log "ok: nosd $NEXT is installed"

make_share after-update || fail "share after the update"
[ "$(share_names)" = "after-update before-update" ] || fail "shares before the rollback: $(share_names)"

log "rolling back to the configuration before the update"
api POST /api/v1/updates/rollback "$(jq -nc --arg s "$snapshot" '{snapshot_id:$s}')" >/dev/null || fail "rollback"
wait_progress rolled_back failed >/dev/null
# nosd restarts to read the restored configuration.
sleep 5
for _ in $(seq 1 60); do
	k -f "$BASE/api/v1/health" >/dev/null 2>&1 && break
	sleep 2
done
k -f "$BASE/api/v1/health" >/dev/null || fail "nosd did not come back after the rollback"
names=""
for _ in $(seq 1 10); do
	names="$(share_names 2>/dev/null || true)"
	[ -n "$names" ] && break
	sleep 3
done
[ "$names" = "before-update" ] || fail "shares after the rollback: '$names' (want before-update)"
log "ok: /etc/nos restored (the share made after the update is gone)"

log "passed"
