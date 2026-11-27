#!/usr/bin/env bash
# Makes an APT repository with a newer nosd for the upgrade test: the nosd
# package from <debs dir> repacked as version <version>+upgradetest1 (same
# files), with its Packages index. Prints the new version.
#
#   tests/upgrade/next-version-repo.sh <debs dir> <repo dir>
#
# Needs dpkg-deb and apt-ftparchive (apt-utils).
set -euo pipefail

DEBS="${1:?folder with the nosd .deb}"
REPO="${2:?output folder}"
deb="$(find "$DEBS" -name 'nosd_*_amd64.deb' | head -n 1)"
[ -n "$deb" ] || { echo "no nosd_*_amd64.deb in $DEBS" >&2; exit 1; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
dpkg-deb -R "$deb" "$work/pkg"
version="$(sed -n 's/^Version: //p' "$work/pkg/DEBIAN/control")"
next="$version+upgradetest1"
sed -i "s/^Version: .*/Version: $next/" "$work/pkg/DEBIAN/control"
mkdir -p "$REPO"
dpkg-deb --root-owner-group -b "$work/pkg" "$REPO/nosd_${next}_amd64.deb" >/dev/null
(cd "$REPO" && apt-ftparchive packages . >Packages)
echo "$next"
