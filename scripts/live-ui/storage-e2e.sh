#!/usr/bin/env bash
# Storage end to end, against the live container (run.sh calls it after
# first-boot setup): nosd and nos-agent create a RAID1 pool on two fresh
# loop devices through the API, as the web interface does, then the test
# checks the pool on the system (mount, profile, subvolumes, /etc/fstab),
# takes and lists a snapshot, scrubs it, refuses a device that is in use,
# and destroys the pool again.
#
#   scripts/live-ui/storage-e2e.sh <nosd base URL> <container> <user> <password>
#
# Needs curl, jq and docker (the container is privileged: loop devices).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BASE="${1:?nosd base URL, e.g. http://127.0.0.1:19000}"
NAME="${2:?container name}"
USER_NAME="${3:?user}"
USER_PASS="${4:?password}"
POOL=e2epool
MOUNT="/srv/pool/$POOL"
LOOPS=()

log() { echo "[storage-e2e] $*"; }
in_box() { docker exec "$NAME" sh -c "$1"; }
fail() {
	log "FAIL: $*"
	log "--- nosd log"
	in_box 'tail -n 40 /var/log/nosd.log | cut -c1-400' 2>/dev/null || true
	log "--- nos-agent log"
	in_box 'tail -n 40 /var/log/nos-agent.log' 2>/dev/null || true
	exit 1
}
cleanup() {
	in_box "umount $MOUNT 2>/dev/null; true" >/dev/null 2>&1 || true
	for dev in "${LOOPS[@]}"; do
		in_box "losetup -d $dev" >/dev/null 2>&1 || true
	done
	in_box "rm -f /var/lib/e2e-*.img" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "signing in as $USER_NAME"
# shellcheck source=session.sh
. "$ROOT/scripts/live-ui/session.sh" || fail "sign-in"

log "two empty disks (loop devices)"
for i in 1 2; do
	# A privileged container sees the host's /dev as it was at start: make
	# the next loop device's node if it is new.
	dev="$(in_box "truncate -s 512M /var/lib/e2e-$i.img && d=\$(losetup -f) && { [ -e \$d ] || mknod \$d b 7 \${d#/dev/loop}; } && losetup \$d /var/lib/e2e-$i.img && echo \$d")" || fail "losetup"
	LOOPS+=("$dev")
	# run.sh detaches whatever is listed here if this script dies first
	in_box "echo $dev >> /run/live-ui-loop"
done
log "disks: ${LOOPS[*]}"
devices="$(printf '%s\n' "${LOOPS[@]}" | jq -R . | jq -sc .)"
spec="$(jq -nc --arg n "$POOL" --arg m "$MOUNT" --argjson d "$devices" \
	'{name:$n, mountpoint:$m, devices:$d, raidData:"raid1", raidMeta:"raid1"}')"

# The existing pool's disk is refused (it is mounted).
pool1_dev="$(in_box "findmnt -no SOURCE /mnt/pool1")" || fail "pool1 is not mounted"
code="$(api_status POST /api/v1/pools/plan-create "$(jq -nc --arg d "$pool1_dev" '{name:"busy", devices:[$d]}')")"
[ "$code" = 409 ] || fail "a pool on the mounted $pool1_dev: $code $(cat "$API_BODY_FILE")"
log "ok: a disk in use is refused"

log "planning $POOL"
plan="$(api POST /api/v1/pools/plan-create "$spec")" || fail "plan-create: $plan"
jq -e '.plan.steps | map(.command) | any(test("mkfs.btrfs"))' <<<"$plan" >/dev/null || fail "no mkfs step in the plan: $plan"
mount_options="$(jq -r .mountOptions <<<"$plan")"
in_box "test ! -e $MOUNT" || fail "planning created $MOUNT"

log "creating $POOL"
tx="$(api POST /api/v1/pools/apply-create "$(jq -c --arg o "$mount_options" '. + {mountOptions:$o, confirm:"CREATE"}' <<<"$spec")" | jq -r .tx_id)"
[ -n "$tx" ] && [ "$tx" != null ] || fail "apply-create gave no transaction"
for _ in $(seq 1 120); do
	status="$(api GET "/api/v1/pools/tx/$tx/status")"
	[ "$(jq -r '.finishedAt // empty' <<<"$status")" != "" ] && break
	sleep 1
done
jq -e .ok <<<"$status" >/dev/null || fail "create transaction: $status"
log "ok: transaction $tx finished"

# On the system: mounted, RAID1, the default subvolumes, a real fstab line.
[ "$(in_box "findmnt -no FSTYPE $MOUNT")" = btrfs ] || fail "$MOUNT is not a mounted btrfs"
in_box "btrfs filesystem df $MOUNT" | grep -q '^Data, RAID1' || fail "data is not RAID1: $(in_box "btrfs filesystem df $MOUNT")"
for sv in data snaps apps; do
	in_box "btrfs subvolume show $MOUNT/$sv" >/dev/null || fail "no subvolume $sv"
done
uuid="$(in_box "findmnt -no UUID $MOUNT")"
line="$(in_box "grep ' $MOUNT ' /etc/fstab")" || fail "no fstab line for $MOUNT"
[[ "$line" == "UUID=$uuid $MOUNT btrfs "*nofail* ]] || fail "fstab line: $line (uuid $uuid)"
log "ok: mounted RAID1 with data, snaps and apps; fstab: $line"

pools="$(api GET /api/v1/pools)"
jq -e --arg m "$MOUNT" 'any(.[]; .mount == $m)' <<<"$pools" >/dev/null || fail "GET /pools lacks $MOUNT: $pools"
log "ok: the pool is listed"

log "snapshot of $MOUNT/data"
code="$(api_status POST /api/v1/backup/snapshots/create "$(jq -nc --arg s "$MOUNT/data" '{subvolumes:[$s], tag:"e2e"}')")"
[ "$code" -lt 300 ] || fail "snapshot create: $code $(cat "$API_BODY_FILE")"
snap=""
for _ in $(seq 1 30); do
	snap="$(api GET /api/v1/backup/snapshots | jq -r --arg m "$MOUNT" '[.snapshots[]? | select((.path // .subvolume // "") | startswith($m))][0].path // empty')"
	[ -n "$snap" ] && break
	sleep 1
done
[ -n "$snap" ] || fail "the snapshot isn't listed: $(api GET /api/v1/backup/snapshots)"
[ "$(in_box "btrfs property get -ts '$snap' ro")" = "ro=true" ] || fail "snapshot $snap isn't read-only"
log "ok: snapshot $snap"

log "scrub"
api POST /api/v1/scrub/start "$(jq -nc --arg m "$MOUNT" '{mount:$m}')" >/dev/null || fail "scrub start"
for _ in $(seq 1 60); do
	scrub="$(api GET "/api/v1/pools/scrub/status?mount=$MOUNT")"
	[ "$(jq -r .status <<<"$scrub")" = finished ] && break
	sleep 1
done
jq -e '.status == "finished" and ((.errors // 0) == 0)' <<<"$scrub" >/dev/null || fail "scrub: $scrub"
log "ok: scrub finished without errors"

log "destroying $POOL"
pool_id="$(jq -r --arg m "$MOUNT" '.[] | select(.mount == $m) | .id' <<<"$pools")"
code="$(api_status POST "/api/v1/pools/$pool_id/plan-destroy" '{"wipe":true,"force":true}')"
[ "$code" = 200 ] || fail "plan-destroy: $code $(cat "$API_BODY_FILE")"
code="$(api_status POST "/api/v1/pools/$pool_id/apply-destroy" '{"confirm":"DESTROY","wipe":true,"force":true}')"
[ "$code" = 200 ] || fail "apply-destroy: $code $(cat "$API_BODY_FILE")"
in_box "! findmnt $MOUNT >/dev/null" || fail "$MOUNT is still mounted"
in_box "! grep -q ' $MOUNT ' /etc/fstab" || fail "the fstab line is still there"
for dev in "${LOOPS[@]}"; do
	[ -z "$(in_box "wipefs -n $dev")" ] || fail "$dev still has a signature"
done
api GET /api/v1/pools | jq -e --arg m "$MOUNT" 'all(.[]; .mount != $m)' >/dev/null || fail "the destroyed pool is still listed"
log "ok: unmounted, out of fstab, disks wiped"

log "passed"
