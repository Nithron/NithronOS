#!/usr/bin/env bash
# NithronSync end to end, against the live container (run.sh calls it after
# first-boot setup): signs in, makes a share, pairs a device, runs the real
# sync daemon (clients/sync-core, paired with the pairing link) on this
# machine against the container's nosd, and checks that files go up, come down, and that a small change to a
# large file goes up as a delta.
#
#   scripts/live-ui/sync-e2e.sh <nosd base URL> <container> <user> <password>
#
# Needs Go with cgo (the client's SQLite), curl, jq and docker.
# SYNC_E2E_KEEP=1 keeps the client's folder and logs (with KEEP=1 for run.sh,
# the container too).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${1:?nosd base URL, e.g. http://127.0.0.1:19000}"
NAME="${2:?container name}"
USER_NAME="${3:?user}"
USER_PASS="${4:?password}"
SHARE=synctest
WAIT_SECS="${SYNC_E2E_WAIT:-60}"
WORK="$(mktemp -d)"
DAEMON_PID=""

log() { echo "[sync-e2e] $*"; }
fail() {
	log "FAIL: $*"
	log "--- daemon output"
	tail -n 60 "$WORK/daemon.log" 2>/dev/null || true
	find "$WORK/home" -name '*.log' -exec tail -n 60 {} + 2>/dev/null || true
	log "--- nosd log"
	docker exec "$NAME" tail -n 60 /var/log/nosd.log 2>/dev/null || true
	exit 1
}
cleanup() {
	if [ -n "$DAEMON_PID" ]; then
		kill "$DAEMON_PID" 2>/dev/null || true
		wait "$DAEMON_PID" 2>/dev/null || true
	fi
	if [ "${SYNC_E2E_KEEP:-0}" = 1 ]; then log "kept the client side in $WORK"; else rm -rf "$WORK"; fi
}
trap cleanup EXIT

# wait_for <what> <command...>: retries the command until it succeeds.
wait_for() {
	local what="$1"
	shift
	for _ in $(seq 1 "$WAIT_SECS"); do
		if "$@" >/dev/null 2>&1; then
			log "ok: $what"
			return 0
		fi
		sleep 1
	done
	fail "$what (after ${WAIT_SECS}s)"
}

log "building the sync daemon"
(cd "$ROOT/clients/sync-core" && CGO_ENABLED=1 go build -o "$WORK/nithron-sync-daemon" ./cmd/daemon)

log "signing in as $USER_NAME"
# The session cookies are Secure; curl won't send those over http, so they
# are passed by hand.
headers="$(curl -fsS -D - -o /dev/null -H 'Content-Type: application/json' \
	-d "$(jq -nc --arg u "$USER_NAME" --arg p "$USER_PASS" '{username:$u,password:$p}')" "$BASE/api/v1/auth/login")"
cookies="$(printf '%s\n' "$headers" | tr -d '\r' | sed -n 's/^[Ss]et-[Cc]ookie: *\([^;]*\).*/\1/p' | paste -sd ';' -)"
csrf="$(printf '%s\n' "$cookies" | tr ';' '\n' | sed -n 's/^ *nos_csrf=//p')"
[ -n "$csrf" ] || fail "no session from the login"
api() { # api <method> <path> [json body]
	curl -fsS -X "$1" -H "Cookie: $cookies" -H "X-CSRF-Token: $csrf" -H 'Content-Type: application/json' \
		${3:+-d "$3"} "$BASE$2"
}

log "creating share $SHARE"
share="$(api POST /api/v1/shares "{\"name\":\"$SHARE\",\"owners\":[\"user:$USER_NAME\"]}")" || fail "share create"
share_path="$(jq -r .path <<<"$share")"
[ "$(jq -r .sync.enabled <<<"$share")" = true ] || fail "a new share isn't syncable: $share"
docker exec "$NAME" test -d "$share_path" || fail "share folder $share_path missing"

log "pairing a device"
reg="$(api POST /api/v1/sync/devices/register '{"device_name":"sync-e2e","device_type":"linux"}')" || fail "device register"
# The pairing link, as the web interface builds it (web/src/lib/pairing.ts).
link="$(jq -rn --arg url "$BASE" --argjson reg "$reg" \
	'"nos://sync?server=\($url|@uri)&token=\($reg.device_token|@uri)&refresh=\($reg.refresh_token|@uri)&name=sync-e2e"')"
# A 5-second poll (the default is 30); --pair fills in the rest.
echo '{"poll_interval_secs": 5, "conflict_policy": "keep_both"}' >"$WORK/config.json"
mkdir -p "$WORK/sync" "$WORK/home"

log "starting the daemon"
HOME="$WORK/home" XDG_CONFIG_HOME="$WORK/home/.config" XDG_DATA_HOME="$WORK/home/.local/share" \
	"$WORK/nithron-sync-daemon" --debug --config "$WORK/config.json" --pair "$link" --folder "$WORK/sync" >"$WORK/daemon.log" 2>&1 &
DAEMON_PID=$!
local_dir="$WORK/sync/$SHARE"
wait_for "the daemon made the share's folder" test -d "$local_dir"

# Up: a file made here reaches the share.
echo "made on the client" >"$local_dir/up.txt"
wait_for "up.txt reached the server" \
	sh -c "[ \"\$(docker exec $NAME cat '$share_path/up.txt')\" = 'made on the client' ]"

# A folder made here reaches the share with its file (WebDAV MKCOL).
mkdir -p "$local_dir/notes/2026"
echo "in a folder" >"$local_dir/notes/2026/todo.txt"
wait_for "notes/2026/todo.txt reached the server" \
	sh -c "[ \"\$(docker exec $NAME cat '$share_path/notes/2026/todo.txt')\" = 'in a folder' ]"

# A file manager lists the share: PROPFIND with the device token as the
# Basic password.
share_id="$(jq -r .id <<<"$share")"
token="$(jq -r .device_token <<<"$reg")"
code="$(curl -s -o "$WORK/propfind.xml" -w '%{http_code}' -u "files:$token" -X PROPFIND -H 'Depth: 1' "$BASE/dav/$share_id/")"
if [ "$code" != 207 ] || ! grep -q 'up.txt' "$WORK/propfind.xml"; then
	fail "PROPFIND as a file manager: $code $(head -c 300 "$WORK/propfind.xml")"
fi
log "ok: a file manager can list the share"

# Down: a file put in the share (as SMB would) reaches the client.
docker exec "$NAME" sh -c "echo 'made on the server' >'$share_path/down.txt' && chown --reference='$share_path' '$share_path/down.txt'"
wait_for "down.txt reached the client" sh -c "[ \"\$(cat '$local_dir/down.txt')\" = 'made on the server' ]"

# Delta: a 9 MiB file goes up whole, then a 100-byte change goes up as a delta.
head -c $((9 << 20)) /dev/urandom >"$local_dir/big.bin"
sum() { sha256sum "$1" | cut -d' ' -f1; }
want="$(sum "$local_dir/big.bin")"
wait_for "big.bin reached the server" \
	sh -c "[ \"\$(docker exec $NAME sha256sum '$share_path/big.bin' | cut -d' ' -f1)\" = '$want' ]"
head -c 100 /dev/urandom | dd of="$local_dir/big.bin" bs=1 seek=$((5 << 20)) conv=notrunc status=none
want="$(sum "$local_dir/big.bin")"
wait_for "the changed big.bin reached the server" \
	sh -c "[ \"\$(docker exec $NAME sha256sum '$share_path/big.bin' | cut -d' ' -f1)\" = '$want' ]"
grep -q '"message":"Delta upload"' "$WORK/daemon.log" || fail "the change went up whole, not as a delta"
log "ok: the change went up as a delta ($(grep -o '"sent_bytes":[0-9]*' "$WORK/daemon.log" | tail -n 1))"

log "passed"
