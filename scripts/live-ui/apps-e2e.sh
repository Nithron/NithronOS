#!/usr/bin/env bash
# Apps end to end, against the live container (run.sh calls it after
# first-boot setup): nosd installs the catalog's whoami app through the API,
# as the install wizard does; nos-agent prepares its folders and the
# container's own Docker runs it. The test checks the app is running, its
# container answers, and it is listed, then leaves it installed so the page
# walk opens an installed app's page.
#
#   scripts/live-ui/apps-e2e.sh <nosd base URL> <container> <user> <password>
#
# Needs curl, jq and docker. The container pulls traefik/whoami.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${1:?nosd base URL, e.g. http://127.0.0.1:19000}"
NAME="${2:?container name}"
USER_NAME="${3:?user}"
USER_PASS="${4:?password}"
APP=whoami
WAIT="${APPS_WAIT:-180}"

log() { echo "[apps-e2e] $*"; }
in_box() { docker exec "$NAME" sh -c "$1"; }
fail() {
	log "FAIL: $*"
	log "--- nosd log"
	in_box 'tail -n 40 /var/log/nosd.log | cut -c1-400' 2>/dev/null || true
	log "--- nos-agent log"
	in_box 'tail -n 40 /var/log/nos-agent.log' 2>/dev/null || true
	log "--- docker"
	in_box 'docker ps -a; tail -n 20 /var/log/dockerd.log' 2>/dev/null || true
	exit 1
}

in_box 'docker info >/dev/null 2>&1' || fail "the container's Docker is not running"

# shellcheck source=session.sh
. "$ROOT/scripts/live-ui/session.sh" || fail "sign-in"

log "installing $APP"
status="$(api_status POST /api/v1/apps/install "$(jq -nc --arg id "$APP" '{id:$id, params:{WHOAMI_PORT:"8080", WHOAMI_NAME:"nOS live test"}}')")"
case "$status" in 2??) ;; *) fail "install answered $status: $(cat "$API_BODY_FILE")" ;; esac

state=""
for _ in $(seq 1 $((WAIT / 3))); do
	state="$(api GET "/api/v1/apps/$APP" 2>/dev/null | jq -r '.status // empty' || true)"
	[ "$state" = running ] && break
	[ "$state" = error ] && fail "the app went to error: $(api GET "/api/v1/apps/$APP" | jq -c .)"
	sleep 3
done
[ "$state" = running ] || fail "the app is '$state' after ${WAIT}s, want running"
log "ok: $APP is running"

in_box "docker ps --filter label=com.docker.compose.project=nos-app-$APP --format '{{.Names}}'" | grep -q . ||
	fail "no container for $APP"
answer="$(in_box 'curl -fsS -m 10 http://127.0.0.1:8080/')" || fail "the app does not answer on port 8080"
grep -q "Hostname" <<<"$answer" || fail "unexpected answer: $answer"
log "ok: its container answers"

api GET /api/v1/apps/installed | jq -e --arg id "$APP" '.items | map(.id) | index($id)' >/dev/null ||
	fail "$APP is not in the installed list: $(api GET /api/v1/apps/installed | jq -c .)"
log "passed ($APP stays installed for the page walk)"
