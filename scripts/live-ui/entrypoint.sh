#!/bin/sh
# Starts nos-agent (root) and nosd (as nos) the way the packages do, minus
# systemd. A Btrfs pool on a loop device gives the storage pages real data.
set -eu

log() { echo "[live-ui] $*"; }

mkdir -p /run/nos && chown nos:nos /run/nos

if [ ! -e /mnt/pool1 ] && img=/var/lib/pool1.img && truncate -s 1G "$img" &&
	dev="$(losetup -f --show "$img" 2>/dev/null)" && mkfs.btrfs -q -L pool1 "$dev" >/dev/null 2>&1; then
	mkdir -p /mnt/pool1 && mount "$dev" /mnt/pool1 && log "Btrfs pool1 on $dev at /mnt/pool1"
else
	log "no loop device: storage pages will show no pools"
fi

/usr/sbin/nos-agent >/var/log/nos-agent.log 2>&1 &
for _ in $(seq 1 50); do [ -S /run/nos-agent.sock ] && break; sleep 0.2; done
[ -S /run/nos-agent.sock ] || { cat /var/log/nos-agent.log; exit 1; }
log "nos-agent ready"

su nos -s /bin/sh -c 'NOS_HTTP_BIND=0.0.0.0:9000 exec /usr/bin/nosd' >/var/log/nosd.log 2>&1 &
log "nosd started"

# Keep the container up while either daemon runs.
wait -n 2>/dev/null || wait
log "a daemon exited"
tail -n 50 /var/log/nos-agent.log /var/log/nosd.log
exit 1
