#!/usr/bin/env bash
# The live UI pass: builds nosd, nos-agent and the web UI, runs the daemons in
# a throwaway Debian container, completes first-boot setup through the API,
# serves the built UI with `vite preview` (proxying /api to the container)
# and walks every page with Playwright (tests/e2e/ui/tests/live-walk.spec.ts).
#
#   scripts/live-ui/run.sh          build, start, walk, stop
#   KEEP=1 scripts/live-ui/run.sh   leave it running for a manual look
#
# Needs docker (privileged containers, for the loop-device Btrfs pool), Go
# and Node. The report lands in tests/e2e/ui/live-report.json.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
NAME="${NOS_LIVE_NAME:-nos-live}"
API_PORT="${NOS_LIVE_API_PORT:-19000}"
UI_PORT="${NOS_LIVE_UI_PORT:-4173}"
ADMIN_USER=admin
ADMIN_PASS='Live-Admin-Pass-1!'
PREVIEW_PID=""

log() { echo "[live-ui] $*"; }
# Removes the container, first unmounting and detaching its loop device: a
# privileged container's loop devices belong to the host and outlive it.
remove_container() {
	docker exec "$NAME" sh -c 'umount /mnt/pool1 2>/dev/null; [ -s /run/live-ui-loop ] && losetup -d "$(cat /run/live-ui-loop)"' >/dev/null 2>&1 || true
	docker rm -f "$NAME" >/dev/null 2>&1 || true
}
cleanup() {
	# preview runs in its own process group (npx leaves vite behind otherwise)
	[ -n "$PREVIEW_PID" ] && kill -- "-$PREVIEW_PID" 2>/dev/null || true
	if [ "${KEEP:-0}" != 1 ]; then remove_container; fi
	rm -rf "$WORK"
}
trap cleanup EXIT

if [ "${1:-}" = "--stop" ]; then
	remove_container
	pkill -f '[v]ite preview --host 127.0.0.1' || true
	log "stopped"
	exit 0
fi

log "building nosd, nos-agent and the web UI"
(cd "$ROOT/backend/nosd" && CGO_ENABLED=0 go build -o "$WORK/nosd" .)
(cd "$ROOT/agent/nos-agent" && CGO_ENABLED=0 go build -o "$WORK/nos-agent" .)
cp "$ROOT/scripts/live-ui/Dockerfile" "$ROOT/scripts/live-ui/entrypoint.sh" "$WORK/"
cp -r "$ROOT/usr/share/nos/apps" "$WORK/apps"
(cd "$ROOT/web" && npm run build --silent >/dev/null)

log "starting $NAME"
remove_container
docker build -q -t nos-live "$WORK" >/dev/null
docker run -d --privileged --name "$NAME" -p "127.0.0.1:$API_PORT:9000" nos-live >/dev/null

api="http://127.0.0.1:$API_PORT/api/v1"
for _ in $(seq 1 60); do curl -fsS "$api/setup/state" >/dev/null 2>&1 && break; sleep 0.5; done
curl -fsS "$api/setup/state" >/dev/null || { docker logs "$NAME"; docker exec "$NAME" cat /var/log/nosd.log; exit 1; }

log "first-boot setup as $ADMIN_USER"
code="$(docker exec "$NAME" sh -c "grep -o 'setup code: [0-9]*' /var/log/nosd.log | tail -n 1 | grep -o '[0-9]*\$'")"
token="$(curl -fsS -H 'Content-Type: application/json' -d "{\"otp\":\"$code\"}" "$api/setup/otp/verify" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
[ -n "$token" ] || { log "no setup token for code '$code'"; exit 1; }
curl -fsS -H 'Content-Type: application/json' -H "Authorization: Bearer $token" \
	-d "{\"token\":\"$token\",\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" "$api/setup/first-admin" >/dev/null
curl -fsS -H 'Content-Type: application/json' -H "Authorization: Bearer $token" -d "{\"token\":\"$token\"}" "$api/setup/complete" >/dev/null

log "serving the UI on http://127.0.0.1:$UI_PORT"
(cd "$ROOT/web" && NOS_API_TARGET="http://127.0.0.1:$API_PORT" exec setsid npx vite preview --host 127.0.0.1 --port "$UI_PORT" --strictPort >"$WORK/preview.log" 2>&1) &
PREVIEW_PID=$!
for _ in $(seq 1 60); do curl -fsS "http://127.0.0.1:$UI_PORT/" >/dev/null 2>&1 && break; sleep 0.5; done
curl -fsS "http://127.0.0.1:$UI_PORT/" >/dev/null || { cat "$WORK/preview.log"; exit 1; }

log "walking the pages"
status=0
(cd "$ROOT/tests/e2e/ui" && BASE_URL="http://127.0.0.1:$UI_PORT" NOS_USER="$ADMIN_USER" NOS_PASS="$ADMIN_PASS" \
	npx playwright test live-walk --reporter=line) || status=$?

if [ "${KEEP:-0}" = 1 ]; then
	log "left running: UI http://127.0.0.1:$UI_PORT (user $ADMIN_USER), logs: docker exec $NAME cat /var/log/nosd.log"
	log "rerun without KEEP (or run: $0 --stop) to remove it and detach its loop device"
	trap - EXIT
	wait "$PREVIEW_PID"
fi
exit "$status"
