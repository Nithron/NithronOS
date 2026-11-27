#!/usr/bin/env bash
# Install a nOS ISO in QEMU, boot the installed disk, and check the
# first-boot flow end to end:
#   - the installer finishes unattended (BIOS or UEFI),
#   - the installed system shows the one-time setup code on its console,
#   - the web API rejects a wrong code and accepts the one on the console.
#
# usage: tests/smoke/iso-install-test.sh <iso> [bios|uefi]
#
# Needs: qemu-system-x86_64, qemu-img, bsdtar (libarchive-tools), gzip, curl,
# and /dev/kvm. For UEFI, OVMF firmware (OVMF_CODE / OVMF_VARS env vars, or
# the usual Debian/Ubuntu/Arch paths). Logs and the disk image go to
# $WORK_DIR (default tests/smoke/logs/<mode>).
set -euo pipefail

ISO="${1:?usage: $0 <iso> [bios|uefi]}"
MODE="${2:-bios}"
WORK_DIR="${WORK_DIR:-tests/smoke/logs/$MODE}"
PORT="${PORT:-18443}"
INSTALL_TIMEOUT="${INSTALL_TIMEOUT:-2400}"
BOOT_TIMEOUT="${BOOT_TIMEOUT:-420}"

mkdir -p "$WORK_DIR"
WORK_DIR="$(cd "$WORK_DIR" && pwd)"
ISO="$(cd "$(dirname "$ISO")" && pwd)/$(basename "$ISO")"
log() { echo "[iso-install-test:$MODE] $*"; }
fail() { log "FAIL: $*"; exit 1; }

QEMU_PID=""
cleanup() { [ -n "$QEMU_PID" ] && kill "$QEMU_PID" 2>/dev/null || true; }
trap cleanup EXIT

firmware=()
if [ "$MODE" = uefi ]; then
	code="${OVMF_CODE:-}"
	vars="${OVMF_VARS:-}"
	for c in /usr/share/OVMF/OVMF_CODE_4M.fd /usr/share/OVMF/OVMF_CODE.fd /usr/share/edk2/x64/OVMF_CODE.4m.fd; do
		[ -z "$code" ] && [ -f "$c" ] && code="$c"
	done
	for v in /usr/share/OVMF/OVMF_VARS_4M.fd /usr/share/OVMF/OVMF_VARS.fd /usr/share/edk2/x64/OVMF_VARS.4m.fd; do
		[ -z "$vars" ] && [ -f "$v" ] && vars="$v"
	done
	[ -n "$code" ] && [ -n "$vars" ] || fail "OVMF firmware not found (set OVMF_CODE and OVMF_VARS)"
	cp "$vars" "$WORK_DIR/vars.fd"
	firmware=(-drive "if=pflash,format=raw,readonly=on,file=$code" -drive "if=pflash,format=raw,file=$WORK_DIR/vars.fd")
fi

# 1. Installer kernel and initrd from the ISO, plus a small overlay that adds
#    the answers a person would give (password, erase confirmation) and a
#    serial console for the installed system. A second cpio archive appended
#    to the initrd replaces /preseed.cfg; the ISO itself is not modified.
log "extracting the installer from $(basename "$ISO")"
rm -rf "$WORK_DIR/di" && mkdir -p "$WORK_DIR/di/overlay"
bsdtar -xf "$ISO" -C "$WORK_DIR/di" install/vmlinuz install/initrd.gz
(cd "$WORK_DIR/di/overlay" && zcat ../install/initrd.gz | bsdtar -xf - preseed.cfg)
cat >> "$WORK_DIR/di/overlay/preseed.cfg" <<'EOF'

### Overrides for the automated install test only
d-i passwd/user-password password Ci-Install-Test-Passw0rd
d-i passwd/user-password-again password Ci-Install-Test-Passw0rd
d-i partman/confirm boolean true
d-i partman/confirm seen true
d-i partman/confirm_nooverwrite boolean true
d-i partman/confirm_nooverwrite seen true
d-i debian-installer/add-kernel-opts string console=tty0 console=ttyS0,115200n8
EOF
(cd "$WORK_DIR/di/overlay" && printf 'preseed.cfg\n' | bsdtar --format newc --uid 0 --gid 0 -cf - -T - | gzip -9) > "$WORK_DIR/di/overlay.gz"
cat "$WORK_DIR/di/install/initrd.gz" "$WORK_DIR/di/overlay.gz" > "$WORK_DIR/di/initrd-test.gz"

qemu_base=(qemu-system-x86_64 -enable-kvm -cpu host -smp 2 -m 3072 "${firmware[@]}"
	-drive "file=$WORK_DIR/disk.qcow2,if=virtio,format=qcow2"
	-netdev "user,id=n0,hostfwd=tcp:127.0.0.1:$PORT-:443" -device virtio-net-pci,netdev=n0
	-display none -no-reboot)

# 2. Install.
rm -f "$WORK_DIR/disk.qcow2"
qemu-img create -f qcow2 "$WORK_DIR/disk.qcow2" 30G >/dev/null
log "installing (timeout ${INSTALL_TIMEOUT}s)"
start=$(date +%s)
# The ISO is attached as a virtio-scsi CD: with OVMF the emulated IDE CD drive
# was not detected by the installer.
"${qemu_base[@]}" -device virtio-scsi-pci,id=scsi0 \
	-drive "file=$ISO,if=none,id=cd0,media=cdrom,readonly=on" -device scsi-cd,drive=cd0,bus=scsi0.0 \
	-kernel "$WORK_DIR/di/install/vmlinuz" -initrd "$WORK_DIR/di/initrd-test.gz" \
	-append "auto=true priority=high DEBIAN_FRONTEND=text console=ttyS0,115200n8 ---" \
	-serial "file:$WORK_DIR/install-serial.log" &
QEMU_PID=$!
while kill -0 "$QEMU_PID" 2>/dev/null; do
	if [ $(( $(date +%s) - start )) -gt "$INSTALL_TIMEOUT" ]; then
		tail -c 3000 "$WORK_DIR/install-serial.log" | tr -cd '\11\12\15\40-\176' || true
		fail "installer did not finish within ${INSTALL_TIMEOUT}s"
	fi
	sleep 5
done
QEMU_PID=""
log "installer finished after $(( $(date +%s) - start ))s"
# A finished install leaves a few GB on the disk; an installer that stopped at
# an error screen and was killed would not get here, but check anyway.
size=$(qemu-img info --output=json "$WORK_DIR/disk.qcow2" | grep -o '"actual-size": [0-9]*' | head -n 1 | grep -o '[0-9]*$')
[ "${size:-0}" -gt 1000000000 ] || fail "disk image is only ${size:-0} bytes; the install did not complete"

# 3. Boot the installed system (no ISO attached) and wait for the setup code
#    on the serial console.
log "booting the installed system (timeout ${BOOT_TIMEOUT}s)"
: > "$WORK_DIR/boot-serial.log"
"${qemu_base[@]}" -serial "file:$WORK_DIR/boot-serial.log" &
QEMU_PID=$!
start=$(date +%s)
code=""
while [ -z "$code" ]; do
	kill -0 "$QEMU_PID" 2>/dev/null || fail "installed system powered off or crashed"
	[ $(( $(date +%s) - start )) -gt "$BOOT_TIMEOUT" ] && break
	code="$(tr -cd '\11\12\15\40-\176' < "$WORK_DIR/boot-serial.log" | grep -oE 'One-time setup code: +[0-9]{3} [0-9]{3}' | tail -n 1 | grep -oE '[0-9]{3} [0-9]{3}' | tr -d ' ' || true)"
	[ -z "$code" ] && sleep 5
done
[ -n "$code" ] || { tail -c 4000 "$WORK_DIR/boot-serial.log" | tr -cd '\11\12\15\40-\176'; fail "no setup code on the console after ${BOOT_TIMEOUT}s"; }
log "console shows setup code $code"

# 4. The web API (through Caddy on 443) must agree with the console.
api="https://127.0.0.1:$PORT/api/v1/setup"
state=""
for _ in $(seq 1 30); do
	state="$(curl -sk -m 5 "$api/state" || true)"
	echo "$state" | grep -q '"otpRequired":true' && break
	sleep 5
done
echo "$state" | grep -q '"otpRequired":true' || fail "setup state: ${state:-no response}"
wrong="000000"; [ "$code" = "$wrong" ] && wrong="111111"
status=$(curl -sk -m 10 -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "{\"otp\":\"$wrong\"}" "$api/otp/verify")
[ "$status" = 400 ] || fail "a wrong code returned HTTP $status (want 400)"
status=$(curl -sk -m 10 -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d "{\"otp\":\"$code\"}" "$api/otp/verify")
[ "$status" = 200 ] || fail "the console's code returned HTTP $status (want 200)"

# 5. nosd's privileged work goes through nos-agent: the share configuration it
#    applies at startup must have reached the real agent (an ISO that shipped a
#    stand-in agent, or a unit sandbox that blocks writing /etc, fails here).
applied=""
for _ in $(seq 1 24); do
	applied="$(tr -cd '\11\12\15\40-\176' < "$WORK_DIR/boot-serial.log" | grep -c 'shares: configuration applied by nos-agent' || true)"
	[ "${applied:-0}" -gt 0 ] && break
	sleep 5
done
if [ "${applied:-0}" -eq 0 ]; then
	tr -cd '\11\12\15\40-\176' < "$WORK_DIR/boot-serial.log" | grep -E 'shares: agent|nos-agent' | tail -n 5
	fail "nosd could not apply the share configuration through nos-agent"
fi
log "nos-agent applied the share configuration"

# 6. Caddy passes WebDAV to nosd (it used to answer /dav with the web
#    interface's HTML).
dav=$(curl -sk -m 10 -o /dev/null -w '%{http_code} %{content_type}' -X PROPFIND "https://127.0.0.1:$PORT/dav/" || true)
case "$dav" in
	*text/html*|"") fail "WebDAV isn't reaching nosd: $dav" ;;
esac
log "WebDAV reaches nosd ($dav)"

# 7. Nothing failed to start at boot (nftables, smbd and a TLS generator
#    used to).
failed=$(tr -cd '\11\12\15\40-\176' < "$WORK_DIR/boot-serial.log" | grep -o 'Failed to start [^.]*' | sed 's/\x1b\[[0-9;]*m//g' | sort -u || true)
if [ -n "$failed" ]; then
	echo "$failed"
	fail "units failed at boot"
fi
log "no unit failed at boot"

log "PASS: installed, console shows the setup code, the API accepts it, nos-agent works"
