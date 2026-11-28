#!/bin/sh
# Starts nos-agent (root) and nosd (as nos) the way the packages do, minus
# systemd. A Btrfs pool on a loop device gives the storage pages real data.
set -eu

log() { echo "[live-ui] $*"; }

mkdir -p /run/nos && chown nos:nos /run/nos

if [ ! -e /mnt/pool1 ] && img=/var/lib/pool1.img && truncate -s 1G "$img" &&
	dev="$(losetup -f --show "$img" 2>/dev/null)"; then
	# The loop device is the host's: run.sh detaches it before removing the
	# container (docker rm leaves it attached otherwise). Recorded at once,
	# so a failed mkfs doesn't leave it behind.
	echo "$dev" > /run/live-ui-loop
	if mkfs.btrfs -q -L pool1 "$dev" >/dev/null 2>&1 && mkdir -p /mnt/pool1 && mount "$dev" /mnt/pool1; then
		log "Btrfs pool1 on $dev at /mnt/pool1"
	else
		log "could not make the pool on $dev: storage pages will show no pools"
	fi
else
	log "no loop device: storage pages will show no pools"
fi

# Docker for apps; /var/lib/docker is a volume (run.sh), not the overlay.
# With cgroup v2, the container's own processes move to a child group first
# so the root group can hand its controllers down to Docker's containers
# (what Docker's dind image does).
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
	mkdir -p /sys/fs/cgroup/init
	xargs -rn1 </sys/fs/cgroup/cgroup.procs >/sys/fs/cgroup/init/cgroup.procs 2>/dev/null || true
	sed -e 's/ / +/g' -e 's/^/+/' </sys/fs/cgroup/cgroup.controllers >/sys/fs/cgroup/cgroup.subtree_control 2>/dev/null || true
fi
dockerd >/var/log/dockerd.log 2>&1 &
for _ in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 0.5; done
if docker info >/dev/null 2>&1; then
	log "Docker ready"
else
	log "Docker did not start (see /var/log/dockerd.log): apps can't be installed"
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
