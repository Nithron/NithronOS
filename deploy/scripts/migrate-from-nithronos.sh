#!/bin/sh
# Moves a system installed as NithronOS to the nOS names. nosd's postinst runs
# it on every configure; once nothing old is left it does nothing.
#
#   /etc/nithronos/tls/*        -> /etc/ssl/nos/  (Caddy reads it; /etc/nos is private)
#   /etc/nithronos/*            -> /etc/nos/
#   /var/lib/nithronos/*        -> /var/lib/nos/
#   unowned nithronos-* units   -> nos-* (enablement kept)
#   old paths in the Caddyfile, unit files, presets and the apt source
#   os-release, issue, motd     -> nOS
#
# Nothing is overwritten: when both the old and the new file exist, the new one
# wins and the old one is left where it is (and reported).
#
# NOS_ROOT runs it against another root (tests): NOS_ROOT=/tmp/x migrate-from-nithronos.sh
set -eu

R="${NOS_ROOT:-}"

log() { echo "nos-migrate: $*"; }

# Old paths and names in file contents -> new ones.
rewrite() {
	sed -i \
		-e 's|/etc/nithronos/tls|/etc/ssl/nos|g' \
		-e 's|/etc/nithronos|/etc/nos|g' \
		-e 's|/usr/share/nithronos|/usr/share/nos|g' \
		-e 's|/usr/lib/nithronos-installer|/usr/lib/nos-installer|g' \
		-e 's|/usr/lib/nithronos|/usr/lib/nos|g' \
		-e 's|/var/lib/nithronos|/var/lib/nos|g' \
		-e 's|/usr/local/bin/nithronos-|/usr/local/bin/nos-|g' \
		-e 's|nithronos-archive-keyring|nos-archive-keyring|g' \
		-e 's|nithronos-\([a-z0-9@-]*\)\.\(service\|timer\|path\|target\)|nos-\1.\2|g' \
		"$1"
}

has_old_names() { grep -q -e nithronos "$1" 2>/dev/null; }

# Moves every entry of $1 into $2 that $2 doesn't have yet, except one named
# $3; removes $1 when it ends up empty.
merge_dir() {
	src="$1" dst="$2" skip="${3:-}"
	[ -d "$src" ] && [ ! -L "$src" ] || return 0
	mkdir -p "$dst"
	for e in "$src"/* "$src"/.[!.]*; do
		[ -e "$e" ] || [ -L "$e" ] || continue
		name="${e##*/}"
		[ "$name" != "$skip" ] || continue
		if [ -d "$e" ] && [ -d "$dst/$name" ]; then
			merge_dir "$e" "$dst/$name"
		elif [ -e "$dst/$name" ] || [ -L "$dst/$name" ]; then
			log "kept $e: $dst/$name already exists"
		else
			mv "$e" "$dst/$name"
			log "moved $e -> $dst/$name"
		fi
	done
	rmdir "$src" 2>/dev/null || log "left $src (not empty)"
}

# Renames $1 to $2 unless $2 exists.
move_file() {
	[ -e "$1" ] || [ -L "$1" ] || return 0
	if [ -e "$2" ] || [ -L "$2" ]; then
		log "kept $1: $2 already exists"
		return 0
	fi
	mkdir -p "$(dirname "$2")"
	mv "$1" "$2"
	log "moved $1 -> $2"
}

unit_exists() {
	for d in /etc/systemd/system /lib/systemd/system /usr/lib/systemd/system; do
		[ -e "$R$d/$1" ] && return 0
	done
	return 1
}

# --- configuration and state --------------------------------------------------

if [ -d "$R/etc/nithronos/tls" ] && [ ! -L "$R/etc/nithronos/tls" ]; then
	merge_dir "$R/etc/nithronos/tls" "$R/etc/ssl/nos"
	chgrp -R caddy "$R/etc/ssl/nos" 2>/dev/null || true
	chmod 0750 "$R/etc/ssl/nos"
	chmod 0640 "$R/etc/ssl/nos"/*.pem 2>/dev/null || true
fi
if [ -d "$R/etc/nithronos" ]; then
	# A tls directory left behind (the new one already had certificates)
	# stays out of /etc/nos.
	merge_dir "$R/etc/nithronos" "$R/etc/nos" tls
	chown -R nos:nos "$R/etc/nos" 2>/dev/null || true
fi
if [ -d "$R/var/lib/nithronos" ]; then
	merge_dir "$R/var/lib/nithronos" "$R/var/lib/nos"
	chown -R nos:nos "$R/var/lib/nos" 2>/dev/null || true
fi

# --- files named after NithronOS ---------------------------------------------

move_file "$R/etc/nftables.d/nithronos.nft" "$R/etc/nftables.d/nos.nft"
move_file "$R/etc/systemd/timesyncd.conf.d/nithronos.conf" "$R/etc/systemd/timesyncd.conf.d/nos.conf"
move_file "$R/etc/avahi/services/nithronos-tm.service" "$R/etc/avahi/services/nos-tm.service"
move_file "$R/etc/default/grub.d/99-nithronos.cfg" "$R/etc/default/grub.d/99-nos.cfg"
move_file "$R/etc/systemd/system-preset/90-nithronos.preset" "$R/etc/systemd/system-preset/90-nos.preset"
move_file "$R/usr/lib/systemd/system-preset/50-nithronos.preset" "$R/usr/lib/systemd/system-preset/50-nos.preset"
move_file "$R/usr/share/keyrings/nithronos-archive-keyring.gpg" "$R/usr/share/keyrings/nos-archive-keyring.gpg"
move_file "$R/etc/apt/sources.list.d/nithronos.list" "$R/etc/apt/sources.list.d/nos.list"
# nosd's tmpfiles.d recreates the link under its new name.
rm -f "$R/etc/issue.d/50-nithronos-setup.issue"

# Helper scripts the ISO put in /usr/local/bin (no package owns them).
for f in "$R"/usr/local/bin/nithronos-*; do
	[ -f "$f" ] || continue
	new="$R/usr/local/bin/nos-${f##*/nithronos-}"
	move_file "$f" "$new"
	if [ ! -e "$f" ] && has_old_names "$new"; then rewrite "$new"; fi
done
# Files in /usr/lib/nithronos that no package owns any more (the packages
# install theirs to /usr/lib/nos).
if [ -d "$R/usr/lib/nithronos" ]; then
	for f in "$R"/usr/lib/nithronos/*; do
		[ -f "$f" ] || continue
		name="${f##*/}"
		if [ -n "$R" ] || ! dpkg -S "/usr/lib/nithronos/$name" >/dev/null 2>&1; then
			move_file "$f" "$R/usr/lib/nos/$name"
			if [ ! -e "$f" ] && has_old_names "$R/usr/lib/nos/$name"; then rewrite "$R/usr/lib/nos/$name"; fi
		fi
	done
	rmdir "$R/usr/lib/nithronos" 2>/dev/null || true
fi

# --- systemd units --------------------------------------------------------------

# Unit files the ISO put in /etc/systemd/system: rename them. Packaged units
# are replaced by the packages themselves.
for f in "$R"/etc/systemd/system/nithronos-*; do
	[ -f "$f" ] && [ ! -L "$f" ] || continue
	new="nos-${f##*/nithronos-}"
	if unit_exists "$new"; then
		rm -f "$f"
		log "removed $f (replaced by $new)"
	else
		move_file "$f" "$R/etc/systemd/system/$new"
	fi
done
# Enablement links: follow the unit to its new name, drop the ones whose unit
# is gone.
for l in "$R"/etc/systemd/system/*.wants/nithronos-* "$R"/etc/systemd/system/*.requires/nithronos-*; do
	[ -L "$l" ] || continue
	old="${l##*/}"
	new="nos-${old#nithronos-}"
	dir="${l%/*}"
	if unit_exists "$new" && [ ! -e "$dir/$new" ]; then
		for d in /etc/systemd/system /lib/systemd/system /usr/lib/systemd/system; do
			if [ -e "$R$d/$new" ]; then
				ln -s "$d/$new" "$dir/$new"
				break
			fi
		done
		log "enabled $new (was $old)"
	fi
	rm -f "$l"
done

# --- old paths inside configuration -------------------------------------------

for f in "$R"/etc/caddy/Caddyfile* "$R"/etc/caddy/conf.d/* \
	"$R"/etc/systemd/system/*.service "$R"/etc/systemd/system/*.timer "$R"/etc/systemd/system/*.path \
	"$R"/etc/systemd/system/*.d/*.conf "$R"/etc/systemd/system-preset/*.preset \
	"$R"/usr/lib/systemd/system-preset/*nos*.preset \
	"$R"/etc/apt/sources.list.d/nos.list; do
	[ -f "$f" ] && [ ! -L "$f" ] || continue
	if has_old_names "$f"; then
		rewrite "$f"
		log "updated paths in $f"
	fi
done

# --- branding -------------------------------------------------------------------

osr="$R/etc/os-release"
if [ -f "$osr" ] && [ ! -L "$osr" ] && grep -q '^ID=nithronos$' "$osr"; then
	sed -i \
		-e 's|^ID=nithronos$|ID=nos|' \
		-e 's|NithronOS|nOS|g' \
		"$osr"
	log "updated $osr"
fi
for f in "$R/etc/issue" "$R/etc/motd"; do
	[ -f "$f" ] && [ ! -L "$f" ] || continue
	if grep -q NithronOS "$f"; then
		sed -i 's|NithronOS|nOS|g' "$f"
		log "updated $f"
	fi
done

exit 0
