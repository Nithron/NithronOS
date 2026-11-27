#!/bin/sh
# Installs nOS's Caddyfile as /etc/caddy/Caddyfile (that file belongs to
# Debian's caddy package, so nos can't ship it). It replaces Debian's
# default and earlier nOS versions; a file someone made their own (without
# the "Managed by nOS" first line) is left alone. The old one is kept as
# Caddyfile.nos-old.
set -eu
SRC=/usr/share/nos/caddy/Caddyfile
DST=/etc/caddy/Caddyfile
mkdir -p /etc/caddy/Caddyfile.d /var/log/caddy
chown caddy:caddy /var/log/caddy 2>/dev/null || true

ours() {
	# This version, or an earlier nOS one (they proxied /api to nosd).
	head -n 1 "$1" | grep -q '^# Managed by nOS' || grep -q 'reverse_proxy 127.0.0.1:9000' "$1"
}
debian_default() {
	grep -q '/usr/share/caddy' "$1" && ! grep -q '127.0.0.1:9000' "$1"
}

if [ -f "$DST" ] && cmp -s "$SRC" "$DST"; then
	exit 0
fi
if [ ! -f "$DST" ] || debian_default "$DST" || ours "$DST"; then
	[ -f "$DST" ] && cp -p "$DST" "$DST.nos-old"
	install -m 0644 -o root -g root "$SRC" "$DST"
	echo "nos: installed $DST"
else
	echo "nos: $DST was changed by hand; leaving it (nOS's version: $SRC)" >&2
fi
