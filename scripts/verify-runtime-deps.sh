#!/usr/bin/env bash
set -euo pipefail

# Verifies that:
#  - Meta package "nos" Depends/Recommends/Suggests include required tools
#  - ISO package list includes the essential runtime tools
#
# Exits non-zero on failure with actionable messages.

CONTROL="packaging/deb/nos/debian/control"
ISO_LIST="packaging/iso/debian/config/package-lists/nos.list.chroot"

REQ_DEP=(btrfs-progs smartmontools cryptsetup util-linux coreutils findutils)
REC_DEP=(nvme-cli)
SUG_DEP=(mdadm lvm2)

err() { printf "❌ %s\n" "$*" >&2; }
ok()  { printf "✅ %s\n" "$*"; }

need_file() {
  local f="$1" label="$2"
  [[ -f "$f" ]] || { err "$label missing: $f"; exit 1; }
}

extract_field_for_pkg() {
  # $1=control path, $2=pkg name, $3=Field (Depends/Recommends/Suggests)
  awk -v pkg="$2" -v field="$3" '
    BEGIN{found=0;grab=0}
    /^[Pp]ackage:[[:space:]]*/{
      pkgline=$2;
      grab=0;
      if ($2==pkg) { found=1; next }
      next
    }
    found && $0 ~ ("^" field ":[[:space:]]*") {
      sub(("^" field ":[[:space:]]*"), "", $0);
      val=$0;
      while (getline > 0) {
        if ($0 ~ /^[A-Za-z-]+:/) break;
        gsub(/^[[:space:]]+/, "", $0);
        val=val "," $0;
      }
      print val;
      exit
    }
  ' "$1"
}

has_dep_in_csv() {
  # $1=csv string, $2=package name; tolerate version constraints and spaces
  local csv="$1" pkg="$2"
  # normalize: collapse spaces
  local norm
  norm="$(printf "%s" "$csv" | tr '\n' ' ' )"
  # match element boundaries: comma or start/end, optional space, pkg, optional version in (), then comma or end
  if grep -Eq "(^|[, ])[[:space:]]*${pkg}([[:space:]]*\([^)]*\))?([, ]|$)" <<<"$norm"; then
    return 0
  else
    return 1
  fi
}

iso_contains_pkg() {
  # $1=list path, $2=pkg
  local list="$1" pkg="$2"
  # strip comments, split on whitespace/newlines
  local tokens
  tokens="$(sed -e 's/#.*$//' "$list" | tr '\n' ' ')"
  grep -Eq "(^|[[:space:]])${pkg}([[:space:]]|$)" <<<"$tokens"
}

main() {
  need_file "$CONTROL" "Debian control"
  need_file "$ISO_LIST" "ISO package list"

  local depends recommends suggests
  depends="$(extract_field_for_pkg "$CONTROL" "nos" "Depends" || true)"
  recommends="$(extract_field_for_pkg "$CONTROL" "nos" "Recommends" || true)"
  suggests="$(extract_field_for_pkg "$CONTROL" "nos" "Suggests" || true)"

  local failed=0

  # Required Depends
  for p in "${REQ_DEP[@]}"; do
    if has_dep_in_csv "$depends" "$p"; then
      ok "Depends contains ${p}"
    else
      err "Depends is missing required ${p}"
      failed=1
    fi
  done

  # Recommends
  for p in "${REC_DEP[@]}"; do
    if has_dep_in_csv "$recommends" "$p"; then
      ok "Recommends contains ${p}"
    else
      err "Recommends is missing ${p}"
      failed=1
    fi
  done

  # Suggests (non-fatal if you prefer; we fail to keep us honest)
  for p in "${SUG_DEP[@]}"; do
    if has_dep_in_csv "$suggests" "$p"; then
      ok "Suggests contains ${p}"
    else
      err "Suggests is missing ${p}"
      failed=1
    fi
  done

  # ISO must include essential runtime pkgs (req + nvme-cli)
  for p in "${REQ_DEP[@]}" nvme-cli; do
    if iso_contains_pkg "$ISO_LIST" "$p"; then
      ok "ISO list includes ${p}"
    else
      err "ISO list is missing ${p} (${ISO_LIST})"
      failed=1
    fi
  done

  if [[ "$failed" -ne 0 ]]; then
    err "One or more checks failed. Please update debian/control and/or ISO package list."
    exit 1
  fi

  ok "All packaging dependency checks passed."
}

main "$@"

