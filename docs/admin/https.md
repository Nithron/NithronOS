# HTTPS (self-signed) on first boot

On first boot, nOS generates a self-signed TLS certificate with SANs for:
- 127.0.0.1
- The device's primary LAN IP
- nithron.os

This enables HTTPS out-of-the-box. HTTP (port 80) redirects to HTTPS. The web UI is served by Caddy and the API is reverse proxied to 127.0.0.1:9000.

Notes:
- The certificate is self-signed. Browsers will warn; you can proceed, trust it, or install your own cert.
- You may add `nithron.os` to your hosts file pointing to the device IP.
- The cert regenerates if the primary IP changes.

Files:
- TLS: `/etc/ssl/nos/cert.pem`, `/etc/ssl/nos/key.pem`
- Caddyfile: `/etc/caddy/Caddyfile`
- TLS generator: `/usr/lib/nos/nos-tls-gen.sh` (systemd unit `nos-tls-gen.service`)


