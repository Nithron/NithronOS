#!/bin/bash
set -euo pipefail

CERT_DIR=/etc/ssl/nos
umask 027
mkdir -p "$CERT_DIR"

# Detect primary IPv4
ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '/src/ {for(i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}' || true)
if [ -z "${ip:-}" ]; then
  ip=$(hostname -I | awk '{print $1}')
fi
ip=${ip:-127.0.0.1}

cert="$CERT_DIR/cert.pem"
key="$CERT_DIR/key.pem"

need_gen=0
if [ ! -s "$cert" ] || [ ! -s "$key" ]; then
  need_gen=1
else
  if ! openssl x509 -in "$cert" -noout -ext subjectAltName 2>/dev/null | grep -q "IP Address:${ip}"; then
    need_gen=1
  fi
fi

if [ "$need_gen" -eq 1 ]; then
  tmpcnf=$(mktemp)
  cat >"$tmpcnf" <<EOF
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
IP.2 = ${ip}
DNS.1 = nos.local
EOF
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
    -days 3650 -sha256 -nodes -subj "/CN=nOS" \
    -keyout "$key" -out "$cert" \
    -addext "subjectAltName=IP:127.0.0.1,IP:${ip},DNS:nos.local"
  rm -f "$tmpcnf"
fi

# Ensure caddy user can traverse the directory and read the files
chgrp -R caddy "$CERT_DIR" 2>/dev/null || true
chmod 750 "$CERT_DIR"
chmod 640 "$CERT_DIR"/*.pem

# Reload caddy if present
if systemctl is-active --quiet caddy 2>/dev/null; then
  systemctl reload caddy || true
fi


