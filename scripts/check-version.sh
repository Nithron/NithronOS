#!/usr/bin/env bash
# Fail if any place that states the release version disagrees with VERSION.
set -euo pipefail
cd "$(dirname "$0")/.."

want="$(tr -d '[:space:]' < VERSION)"
deb="${want/-/\~}-1" # 0.9.5-pre-alpha -> 0.9.5~pre-alpha-1 (sorts before 0.9.5)
fail=0

check() { # <file> <found>
  if [ "$2" != "$3" ]; then
    echo "::error file=$1::version '$2' does not match VERSION ('$3')"
    fail=1
  fi
}

check web/package.json "$(node -p "require('./web/package.json').version")" "$want"
check backend/nosd/openapi.yaml "$(sed -n 's/^  version: *//p' backend/nosd/openapi.yaml | head -1)" "$want"
check docs-site/mkdocs.yml "$(sed -n 's/^  version: *v//p' docs-site/mkdocs.yml | head -1)" "$want"
for f in packaging/deb/*/debian/changelog; do
  check "$f" "$(head -1 "$f" | sed -n 's/^[^ ]* (\([^)]*\)).*/\1/p')" "$deb"
done

[ "$fail" -eq 0 ] && echo "version $want is consistent"
exit "$fail"
