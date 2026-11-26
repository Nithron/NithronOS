#!/bin/bash
set -euo pipefail

log() { logger -t nos-deps "$*"; }

report_version() {
  local cmd=$1
  local ver_cmd=$2
  if command -v "$cmd" >/dev/null 2>&1; then
    eval "$ver_cmd" | head -n1 | sed 's/^/version: /' | while read -r line; do log "$cmd $line"; done
  else
    log "$cmd not installed"
  fi
}

report_version btrfs 'btrfs --version'
report_version smartctl 'smartctl -V'
report_version cryptsetup 'cryptsetup --version'
report_version mount 'mount --version'
report_version mdadm 'mdadm --version'
report_version lvm 'lvm version'

install -d -m 0755 /var/lib/nos
touch /var/lib/nos/deps-reported


