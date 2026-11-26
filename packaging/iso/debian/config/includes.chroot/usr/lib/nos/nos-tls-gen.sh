#!/bin/sh
set -eu

PRIMARY_IP="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") {print $(i+1); exit}}' || hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$PRIMARY_IP" ] || PRIMARY_IP=127.0.0.1

TLS_DIR=/etc/ssl/nos
install -d -m 0750 -o root -g caddy "$TLS_DIR"

CERT="$TLS_DIR/cert.pem"
KEY="$TLS_DIR/key.pem"

NEED_GEN=0
if [ ! -s "$CERT" ]; then
  NEED_GEN=1
elif ! openssl x509 -in "$CERT" -noout -text 2>/dev/null | grep -q "$PRIMARY_IP"; then
  NEED_GEN=1
fi

if [ "$NEED_GEN" -eq 1 ]; then
  TMPCONF="$(mktemp)"
  cat >"$TMPCONF" <<EOF
[ req ]
default_bits       = 2048
prompt             = no
default_md         = sha256
distinguished_name = dn
req_extensions     = req_ext

[ dn ]
CN = nos.local

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = nos.local
IP.1  = 127.0.0.1
IP.2  = $PRIMARY_IP
EOF
  openssl req -x509 -nodes -days 825 -newkey rsa:2048 -keyout "$KEY" -out "$CERT" -config "$TMPCONF" >/dev/null 2>&1 || true
  rm -f "$TMPCONF"
  echo "[nos-tls-gen] generated cert for IP=$PRIMARY_IP"
else
  echo "[nos-tls-gen] reusing cert for IP=$PRIMARY_IP"
fi

# Ensure caddy user can traverse the directory and read the files
chgrp -R caddy "$TLS_DIR" 2>/dev/null || true
chmod 750 "$TLS_DIR" || true
chmod 640 "$TLS_DIR"/*.pem 2>/dev/null || true


