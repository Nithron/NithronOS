#!/usr/bin/env bash
set -euo pipefail

# Resolve repo paths robustly
SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PKG_ROOT="$REPO_ROOT/packaging/deb"
DIST_DIR="$REPO_ROOT/dist/deb"

echo "[info] REPO_ROOT=$REPO_ROOT"
echo "[info] PKG_ROOT=$PKG_ROOT"
mkdir -p "$DIST_DIR"

# List of package folders under packaging/deb/
packages=(nosd nos-agent nos-web nos-shares nos-apps-runtime nos-apps nos)

for pkg in "${packages[@]}"; do
  DIR="$PKG_ROOT/$pkg"
  if [[ ! -d "$DIR" ]]; then
    echo "[warn] Skipping missing package dir: $DIR" >&2
    continue
  fi
  echo "[build] $DIR"
  (cd "$DIR" && dpkg-buildpackage -us -uc -b)
done

# Collect artifacts (built one level above each package dir)
echo "[collect] moving .deb/.changes/.buildinfo into $DIST_DIR"
find "$PKG_ROOT" -maxdepth 2 -type f \( -name "*.deb" -o -name "*.changes" -o -name "*.buildinfo" \) -exec mv -v {} "$DIST_DIR"/ \;

echo "[done] artifacts in $DIST_DIR"


