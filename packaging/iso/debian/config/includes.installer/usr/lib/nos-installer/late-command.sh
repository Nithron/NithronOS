#!/bin/sh
# nOS installer: final steps on the installed system (/target).
#
# Run by the preseed's late_command inside the Debian installer (busybox sh),
# after live-installer has copied the live system to the disk. Any failure
# makes the installer show an error instead of finishing with a broken system.
set -eu

log() { echo "nos-late: $*"; logger -t nos-late "$*" 2>/dev/null || true; }
fail() { log "ERROR: $*"; exit 1; }

T=/target

# 1. The nOS services must be on the disk. If the live image was built
#    without them, stop here rather than produce a system with no web UI.
for f in /usr/bin/nosd /usr/sbin/nos-agent /usr/share/nos/web/index.html; do
	[ -e "$T$f" ] || fail "$f is missing from the installed system; the ISO was built without the nOS packages"
done
log "nOS packages present"

# 2. APT sources for updates, including Debian security updates. contrib is
#    needed for ZFS. Docker's repository list, if the live system has one, is
#    kept as is in sources.list.d.
cat > "$T/etc/apt/sources.list" <<'EOF'
deb http://deb.debian.org/debian bookworm main contrib non-free-firmware
deb http://deb.debian.org/debian bookworm-updates main contrib non-free-firmware
deb http://security.debian.org/debian-security bookworm-security main contrib non-free-firmware
EOF
log "APT sources written"

# 2b. Btrfs compression. The installer's Btrfs "compress" option is only an
#     on/off switch (it means zlib), so set the algorithm the layout intends.
if grep -q '[[:space:]]btrfs[[:space:]]' "$T/etc/fstab"; then
	awk 'BEGIN { OFS = "\t" }
	     $3 == "btrfs" && $4 !~ /compress=zstd/ {
	         if ($4 ~ /compress(=[^,]*)?/) sub(/compress(=[^,]*)?/, "compress=zstd:3", $4)
	         else $4 = $4 ",compress=zstd:3"
	     }
	     { print }' "$T/etc/fstab" > "$T/etc/fstab.nos" &&
		mv "$T/etc/fstab.nos" "$T/etc/fstab"
	log "Btrfs mounts use compress=zstd:3"
fi

# 3. Services that must start on first boot. nosd shows the first-boot setup
#    code on the console; nos-issue-reload.path refreshes the login prompts.
for unit in nosd.service nos-agent.service nos-firewall.service nos-certgen.service caddy.service; do
	in-target systemctl enable "$unit" || fail "cannot enable $unit"
done
# (the nosd package also enables nos-issue-reload.path itself)
for unit in nos-issue-reload.path docker.service ssh.service; do
	in-target systemctl enable "$unit" 2>/dev/null || log "optional unit $unit not enabled"
done

# 4. The console administrator may use sudo (and docker when present).
user="$(debconf-get passwd/username 2>/dev/null || true)"
if [ -n "$user" ]; then
	in-target usermod -aG sudo "$user" || log "cannot add $user to sudo"
	if in-target getent group docker >/dev/null 2>&1; then
		in-target usermod -aG docker "$user" || log "cannot add $user to docker"
	fi
fi

# 5. Boot menu name.
mkdir -p "$T/etc/default/grub.d"
echo 'GRUB_DISTRIBUTOR="nOS"' > "$T/etc/default/grub.d/nos.cfg"
in-target update-grub || log "update-grub failed; the boot menu keeps the Debian name"

log "done"
