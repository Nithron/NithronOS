#!/usr/bin/env bash
set -euo pipefail

# run as root if available; otherwise prefix with sudo
SUDO_CMD=""
if [ "${EUID:-$(id -u)}" -ne 0 ]; then
  SUDO_CMD="sudo"
fi

DEB_DIR="${1:-packaging/iso/local-debs}"

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$SCRIPT_DIR/debian"
OUT_DIR="$SCRIPT_DIR/../../dist/iso"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Fetch Debian Installer files
echo "[iso] Fetching Debian Installer kernel and initrd..."
if [ -x "$SCRIPT_DIR/scripts/fetch-di.sh" ]; then
    bash "$SCRIPT_DIR/scripts/fetch-di.sh" || {
        echo "::error::Failed to fetch Debian Installer files"
        exit 1
    }
else
    echo "::warning::fetch-di.sh not found or not executable, skipping d-i fetch"
fi

# --- begin logo wiring ---
LOGO_SRC=""
for p in \
  "$REPO_ROOT/assets/nithronos-logo-mark.png" \
  "$REPO_ROOT/assets/brand/nithronos-logo-mark.png"
do
  if [ -f "$p" ]; then LOGO_SRC="$p"; break; fi
done

if [ -n "$LOGO_SRC" ]; then
  echo "[iso] branding with logo: $LOGO_SRC"

  # Debian Installer graphics
  mkdir -p "$PROFILE_DIR/config/includes.installer/usr/share/graphics"
  # Logo (light + dark)
  cp "$LOGO_SRC" "$PROFILE_DIR/config/includes.installer/usr/share/graphics/logo_debian.png"
  cp "$LOGO_SRC" "$PROFILE_DIR/config/includes.installer/usr/share/graphics/logo_debian_dark.png" || true

  # Splash: try convert to 640x480, fallback to raw
  SPL_OUT="$PROFILE_DIR/config/includes.installer/usr/share/graphics/splash.png"
  if command -v convert >/dev/null 2>&1; then
    convert "$LOGO_SRC" -resize 640x480 -background black -gravity center -extent 640x480 "$SPL_OUT" || cp "$LOGO_SRC" "$SPL_OUT"
  else
    cp "$LOGO_SRC" "$SPL_OUT"
  fi

  # GRUB background (UEFI)
  mkdir -p "$PROFILE_DIR/config/includes.binary/boot/grub"
  cp "$SPL_OUT" "$PROFILE_DIR/config/includes.binary/boot/grub/splash.png" || cp "$LOGO_SRC" "$PROFILE_DIR/config/includes.binary/boot/grub/splash.png"

  # ISOLINUX background (BIOS)
  mkdir -p "$PROFILE_DIR/config/includes.binary/isolinux"
  cp "$SPL_OUT" "$PROFILE_DIR/config/includes.binary/isolinux/splash.png" || cp "$LOGO_SRC" "$PROFILE_DIR/config/includes.binary/isolinux/splash.png"
fi
# --- end logo wiring ---

# Ensure apt includes directories exist for pinned sources
mkdir -p "$PROFILE_DIR/config/includes.chroot/etc/apt" "$PROFILE_DIR/config/includes.binary/etc/apt"
mkdir -p "$PROFILE_DIR/config/archives"
mkdir -p "$PROFILE_DIR/config/hooks/early"

# Force-correct apt sources early inside chroot to avoid bookworm/updates and duplicates
FORCE_SOURCES_HOOK="$PROFILE_DIR/config/hooks/early/00-force-sources.chroot"
cat > "$FORCE_SOURCES_HOOK" <<'EOS'
#!/bin/sh
set -e
cat > /etc/apt/sources.list <<'EOF'
deb http://deb.debian.org/debian bookworm main contrib non-free-firmware
deb http://deb.debian.org/debian bookworm-updates main contrib non-free-firmware
deb http://security.debian.org/debian-security bookworm-security main contrib non-free-firmware
EOF
# Remove any additional lists to prevent duplicates
rm -f /etc/apt/sources.list.d/* 2>/dev/null || true
apt-get update || true
EOS
chmod +x "$FORCE_SOURCES_HOOK"

echo "[iso] using debs from: $DEB_DIR"
mkdir -p "$OUT_DIR"

# Branding files (os-release, issue, motd) from the repo's VERSION
bash "$SCRIPT_DIR/scripts/gen-branding.sh" "$REPO_ROOT/VERSION" "$PROFILE_DIR/config/includes.chroot"

# Prepare live-build working dir
mkdir -p "$PROFILE_DIR/config/includes.chroot/root/debs"
mkdir -p "$PROFILE_DIR/config/hooks/normal"

# Stage local debs (from CI artifact)
if ls "$DEB_DIR"/*.deb 1>/dev/null 2>&1; then
  cp -v "$DEB_DIR"/*.deb "$PROFILE_DIR/config/includes.chroot/root/debs/" || true
else
  echo "::error::No .deb files found in $DEB_DIR"
  exit 1
fi

# Make all hooks executable
chmod +x "$PROFILE_DIR/config/hooks/normal/"*.chroot 2>/dev/null || true
chmod +x "$PROFILE_DIR/config/hooks/normal/"*.binary 2>/dev/null || true

# Hook to install staged debs inside chroot during build
HOOK="$PROFILE_DIR/config/hooks/normal/20-install-local-debs.chroot"
cat > "$HOOK" <<'EOS'
#!/bin/sh
set -e
if ls /root/debs/*.deb 1>/dev/null 2>&1; then
  # Use non-interactive mode to avoid config file prompts
  export DEBIAN_FRONTEND=noninteractive
  export DEBCONF_NONINTERACTIVE_SEEN=true
  
  # Install packages, keeping existing config files
  dpkg --force-confold -i /root/debs/*.deb || apt-get -y -f install -o Dpkg::Options::="--force-confold" || true
  
  # Try again for nos-web specifically if first pass failed due to conflicts
  if ! dpkg -l | grep -qi '^ii\s\+nos-web'; then
    apt-get -y install -o Dpkg::Options::="--force-confold" ./root/debs/nos-web_*.deb 2>/dev/null || true
  fi
fi
# Enable Caddy if present
systemctl enable caddy || true
# Verify web assets exist
if [ ! -f /usr/share/nithronos/web/index.html ]; then
  echo "::warning::/usr/share/nithronos/web/index.html missing in chroot"
  echo "[iso] debs staged in chroot:"
  ls -1 /root/debs || true
  echo "[iso] dpkg packages matching nos-web:"
  dpkg -l | grep -i nos-web || true
  echo "[iso] contents of /usr/share/nithronos (if any):"
  ls -lah /usr/share/nithronos || true
fi
EOS
chmod +x "$HOOK"

# No longer need sed-based fix hooks; sources are pinned via includes and archives config

# Run live-build
pushd "$PROFILE_DIR" >/dev/null

# Clean previous outputs
${SUDO_CMD} lb clean --purge || true
rm -rf chroot/ cache/* || true

# Configure build (Debian bookworm, ISO-hybrid)
DEBIAN_MIRROR="http://deb.debian.org/debian"
ARCH="${ISO_ARCH:-amd64}"

# Disable LB security mirrors to avoid legacy bookworm/updates
export LB_MIRROR_CHROOT_SECURITY=""
export LB_MIRROR_BINARY_SECURITY=""
export LB_SECURITY="false"

# Enable serial for syslinux (BIOS boot menu) so early bootloader output goes to serial
export LB_SYSLINUX_SERIAL="0 115200"

## No kernel stage overrides; rely on package lists (linux-image-amd64) and lb defaults

${SUDO_CMD} lb config \
  --mode debian \
  --distribution bookworm \
  --architectures "${ARCH}" \
  --binary-images iso-hybrid \
  --apt-recommends true \
  --apt-indices false \
  --debian-installer live \
  --archive-areas "main contrib non-free-firmware" \
  --mirror-bootstrap "$DEBIAN_MIRROR" \
  --mirror-chroot   "$DEBIAN_MIRROR" \
  --mirror-binary   "$DEBIAN_MIRROR" \
  --linux-flavours "${ARCH}" \
  --linux-packages "linux-image" \
  --initramfs "live-boot"

## Do not persist LB_LINUX_* overrides; kernel handled via package lists

# Persist only LB_SYSLINUX_SERIAL into profile (clean any prior entries)
for f in "$PROFILE_DIR/config/common" "$PROFILE_DIR/config/chroot"; do
  [ -f "$f" ] && sed -i -E '/^LB_BOOTAPPEND_(LIVE|INSTALL)=/d; /^LB_SYSLINUX_SERIAL=/d' "$f"
done
printf '%s\n' 'LB_SYSLINUX_SERIAL="0 115200"' >> "$PROFILE_DIR/config/common"
printf '%s\n' 'LB_SYSLINUX_SERIAL="0 115200"' >> "$PROFILE_DIR/config/chroot"

# Remove any stale security lines live-build might inject
sed -i '/security\.debian\.org.*bookworm\/updates/d' "$PROFILE_DIR"/config/* 2>/dev/null || true

# Normalize LB_SECURITY in any persisted live-build config and remove legacy --security flags
sed -i -E 's/^LB_SECURITY=.*$/LB_SECURITY="false"/' "$PROFILE_DIR"/config/* 2>/dev/null || true
sed -i '/--security/d' "$PROFILE_DIR"/config/* 2>/dev/null || true

# Make auto/config executable if it exists to silence warnings
if [ -f "$PROFILE_DIR/auto/config" ]; then chmod +x "$PROFILE_DIR/auto/config"; fi

# Ensure previously persisted invalid LB_SECURITY is corrected in profile files
for f in "$PROFILE_DIR/config/common" "$PROFILE_DIR/config/chroot"; do
  [ -f "$f" ] && sed -i -E 's/^LB_SECURITY=.*$/LB_SECURITY="false"/' "$f"
done

# Build ISO (LB assumes non-interactive)
export DEBIAN_FRONTEND=noninteractive
echo "[iso] Starting live-build..."
${SUDO_CMD} lb build 2>&1 | tee build.log || {
    echo "[iso] Build failed. Checking for kernel files..."
    echo "[iso] Contents of binary/live (if exists):"
    ls -la binary/live/ 2>/dev/null || echo "binary/live does not exist"
    echo "[iso] Contents of chroot/boot (if exists):"
    ls -la chroot/boot/vmlinuz* chroot/boot/initrd* 2>/dev/null || echo "No kernel files in chroot/boot"
    exit 1
}

# Default output path from live-build
ISO_SRC="live-image-${ARCH}.hybrid.iso"

# Name the ISO as: NithronOS - <arch> - <tag>.iso
TAG="${ISO_TAG:-${GITHUB_REF_NAME:-dev}}"
ISO_DST="$OUT_DIR/NithronOS - ${ARCH} - ${TAG}.iso"
[ -f "$ISO_SRC" ] || { echo "::error::ISO not found at $ISO_SRC"; exit 1; }
mv -v "$ISO_SRC" "$ISO_DST"

popd >/dev/null
echo "[iso] built $ISO_DST"

# Post-build smoke test: verify d-i payloads and boot entries/flags
echo "[iso] Running post-build smoke test..."
MNT_DIR="$(mktemp -d)"
trap 'umount "$MNT_DIR" >/dev/null 2>&1 || true; rmdir "$MNT_DIR" >/dev/null 2>&1 || true' EXIT
${SUDO_CMD} mount -o loop "$ISO_DST" "$MNT_DIR"

missing=0
for f in install.amd/vmlinuz install.amd/initrd.gz; do
  if [ ! -f "$MNT_DIR/$f" ]; then
    echo "::error::Missing $f in ISO"
    missing=1
  fi
done

## Check GRUB UEFI config — scope checks to installer kernel lines only
GRUB_CFG="$(find "$MNT_DIR" -maxdepth 4 -type f -name grub.cfg | head -n1)"
if [ -f "$GRUB_CFG" ]; then
  echo "[iso] Checking GRUB config at $GRUB_CFG"
  # Extract only installer linux lines
  installer_linux_lines="$(grep -E '^\s*linux\s+/install\.amd/vmlinuz' "$GRUB_CFG" || true)"
  if [ -z "$installer_linux_lines" ]; then
    echo "::error::No installer linux lines found in GRUB config"
    missing=1
  else
    echo "$installer_linux_lines" | grep -q 'DEBIAN_FRONTEND=text' || { echo "::error::GRUB installer missing DEBIAN_FRONTEND=text"; missing=1; }
    echo "$installer_linux_lines" | grep -q 'fb=false' || { echo "::error::GRUB installer missing fb=false"; missing=1; }
    echo "$installer_linux_lines" | grep -q 'nomodeset' || { echo "::error::GRUB installer missing nomodeset"; missing=1; }
    if echo "$installer_linux_lines" | grep -qE 'quiet|splash'; then
      echo "::error::Installer entries must not include quiet/splash"
      missing=1
    fi
  fi
else
  echo "::warning::Could not locate GRUB config inside ISO; skipping GRUB checks"
fi

# Check BIOS isolinux config if present (scope to installer APPEND lines)
for CFG in $(find "$MNT_DIR" -type f -name 'isolinux.cfg' -o -name 'txt.cfg'); do
  echo "[iso] Checking BIOS config at $CFG"
  installer_append_lines="$(grep -E '^\s*APPEND\s+.*initrd=/install\.amd/initrd\.gz' "$CFG" || true)"
  if [ -z "$installer_append_lines" ]; then
    echo "::error::No installer APPEND lines found in BIOS config"
    missing=1
  else
    echo "$installer_append_lines" | grep -q 'DEBIAN_FRONTEND=text' || { echo "::error::BIOS installer missing DEBIAN_FRONTEND=text"; missing=1; }
    echo "$installer_append_lines" | grep -q 'fb=false' || { echo "::error::BIOS installer missing fb=false"; missing=1; }
    echo "$installer_append_lines" | grep -q 'nomodeset' || { echo "::error::BIOS installer missing nomodeset"; missing=1; }
    if echo "$installer_append_lines" | grep -qE 'quiet|splash'; then
      echo "::error::Installer BIOS entries must not include quiet/splash"
      missing=1
    fi
  fi
done

${SUDO_CMD} umount "$MNT_DIR" || true
rmdir "$MNT_DIR" || true
trap - EXIT

if [ "$missing" -ne 0 ]; then
  echo "[iso] Post-build smoke test FAILED"
  exit 1
fi

echo "[iso] Post-build smoke test PASSED"
