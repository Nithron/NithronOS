#!/usr/bin/env bash
# Writes the nOS branding files (os-release, issue, motd) into the
# live-build chroot includes, using the version from the repo's VERSION file.
# The installer copies the live filesystem, so installed systems get the same
# files.
#
# usage: gen-branding.sh <VERSION file> <includes.chroot dir>
set -euo pipefail

version="$(tr -d '[:space:]' < "$1")"
root="$2"
version_id="${version%%-*}" # 0.9.5-pre-alpha -> 0.9.5

mkdir -p "$root/etc"

# VERSION_CODENAME stays the Debian codename: tools such as Docker's apt
# setup read it to pick the right repository.
cat > "$root/etc/os-release" <<OSR
PRETTY_NAME="nOS $version"
NAME="nOS"
VERSION_ID="$version_id"
VERSION="$version"
VERSION_CODENAME=trixie
ID=nos
ID_LIKE=debian
HOME_URL="https://github.com/Nithron/nOS"
SUPPORT_URL="https://github.com/Nithron/nOS/issues"
BUG_REPORT_URL="https://github.com/Nithron/nOS/issues"
OSR

# agetty escapes: \n hostname, \l tty. The first-boot setup code is added
# below this by nosd through /etc/issue.d/50-nos-setup.issue.
cat > "$root/etc/issue" <<'ISSUE'

  nOS @VERSION@  -  \n  (\l)

ISSUE
sed -i "s/@VERSION@/$version/" "$root/etc/issue"

cat > "$root/etc/motd" <<MOTD

  nOS $version
  Manage this system from the web interface: https://<this machine's address>/
  Documentation: https://github.com/Nithron/nOS/tree/main/docs

MOTD
