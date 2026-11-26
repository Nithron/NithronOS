#!/bin/bash
set -euo pipefail

out_dir=/var/lib/nos/health/smart
mkdir -p "$out_dir"

list_json=$(lsblk --bytes --json -O -o NAME,PATH,TYPE,RM)
devices=$(echo "$list_json" | jq -r '.blockdevices[] | select(.type=="disk" and (.rm|not or .rm==false)) | .path // ("/dev/"+.name)')
for dev in $devices; do
  base=$(basename "$dev" | tr '/' '_')
  tmp=$(mktemp)
  if smartctl -H -A -j "$dev" >"$tmp" 2>/dev/null; then
    :
  elif smartctl -H -A -j -d nvme "$dev" >"$tmp" 2>/dev/null; then
    :
  else
    echo '{"error":"smartctl failed"}' >"$tmp"
  fi
  mv "$tmp" "$out_dir/$base.json"
done


