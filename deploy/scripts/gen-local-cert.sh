#!/bin/bash
# The web interface's self-signed certificate (Caddy serves it on 443).
#
# The private key is made on this machine, once, and never shipped: the ISO
# used to create one when it was built, so every system installed from the
# same image shared it (a key from such an image is replaced). The
# certificate names every address the machine has now, its hostname,
# <hostname>.local and nos.local; when an address is missing it is reissued
# with the same key. Runs at boot (nos-certgen.service, before Caddy).
set -euo pipefail

CERT_DIR=${NOS_CERT_DIR:-/etc/ssl/nos}
cert="$CERT_DIR/cert.pem"
key="$CERT_DIR/key.pem"
umask 027
mkdir -p "$CERT_DIR"

# The image's shared key: its certificate said CN = nOS-Placeholder.
if [ -s "$cert" ] && openssl x509 -in "$cert" -noout -subject 2>/dev/null | grep -q 'nOS-Placeholder'; then
	echo "nos-certgen: replacing the key that came with the installation image"
	rm -f "$cert" "$key"
fi

if [ ! -s "$key" ]; then
	openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$key.new"
	mv "$key.new" "$key"
	rm -f "$cert"
	echo "nos-certgen: new private key"
fi

# What the certificate must name: loopback, every global address, names.
host=$(hostname -s 2>/dev/null || hostname)
names=("DNS:nos.local" "DNS:localhost" "IP:127.0.0.1" "IP:::1")
if [ -n "$host" ] && [ "$host" != "localhost" ]; then
	names+=("DNS:$host" "DNS:$host.local")
fi
while read -r addr; do
	[ -n "$addr" ] && names+=("IP:$addr")
done < <(ip -o addr show scope global 2>/dev/null | awk '{split($4, a, "/"); print a[1]}')

# OpenSSL prints IPv6 addresses expanded (::1 as 0:0:0:0:0:0:0:1): expand
# before comparing, or a machine with IPv6 would get a new certificate at
# every boot.
expand6() {
	local a=$1 left right nl=0 nr=0 mid="" i
	if [[ $a == *::* ]]; then
		left=${a%%::*}
		right=${a##*::}
		[ -n "$left" ] && nl=$(awk -F: '{print NF}' <<<"$left")
		[ -n "$right" ] && nr=$(awk -F: '{print NF}' <<<"$right")
		for ((i = 0; i < 8 - nl - nr; i++)); do mid+="0:"; done
		a="${left:+$left:}${mid}${right}"
		a=${a%:}
	fi
	awk -F: '{for (i = 1; i <= NF; i++) {g = tolower($i); sub(/^0+/, "", g); if (g == "") g = "0"; printf "%s%s", (i > 1 ? ":" : ""), g} print ""}' <<<"$a"
}
canon() {
	case $1 in
	IP:*:*) echo "IP:$(expand6 "${1#IP:}")" ;;
	*) echo "$1" ;;
	esac
}

missing=0
if [ -s "$cert" ]; then
	# Lowercase both sides: OpenSSL prints IPv6 hex in capitals.
	have=$(openssl x509 -in "$cert" -noout -ext subjectAltName 2>/dev/null | tr ',' '\n' | sed 's/^ *//; s/IP Address:/IP:/' | tr '[:upper:]' '[:lower:]' || true)
	for n in "${names[@]}"; do
		# Exact names: 10.0.0.1 must not match 10.0.0.12.
		if ! printf '%s\n' "$have" | grep -qxF "$(canon "$n" | tr '[:upper:]' '[:lower:]')"; then
			missing=1
			break
		fi
	done
else
	missing=1
fi

if [ "$missing" -eq 1 ]; then
	san=$(IFS=,; echo "${names[*]}")
	openssl req -x509 -new -key "$key" -days 825 -sha256 -subj "/CN=${host:-nos}" \
		-addext "subjectAltName=$san" -out "$cert.new"
	mv "$cert.new" "$cert"
	echo "nos-certgen: certificate for $san"
fi

chgrp -R caddy "$CERT_DIR" 2>/dev/null || true
chmod 750 "$CERT_DIR"
chmod 640 "$cert" "$key"
if [ "$missing" -eq 1 ] && systemctl is-active --quiet caddy 2>/dev/null; then
	systemctl reload caddy || true
fi
