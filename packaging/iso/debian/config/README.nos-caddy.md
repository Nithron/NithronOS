# nOS Caddy Configuration

This document describes the Caddy web server configuration for nOS, providing automatic HTTPS with self-signed certificates and reliable service management.

## Overview

nOS uses Caddy as the primary web server to:
- Serve the web UI from `/usr/share/nos/web`
- Reverse proxy API requests to the nosd service at `127.0.0.1:9000`
- Automatically redirect HTTP to HTTPS
- Provide HTTPS using Caddy's internal CA (`tls internal`)
- Start automatically on boot with proper service ordering

## Configuration

### Caddyfile Location
- **Primary**: `/etc/caddy/Caddyfile`
- **ISO Include**: `packaging/iso/debian/config/includes.chroot/etc/caddy/Caddyfile`

The configuration is self-contained (no imports) to avoid dependency issues.

### Key Features

1. **Automatic HTTPS**: Uses `tls internal` to generate self-signed certificates automatically
2. **HTTP Redirect**: All HTTP traffic (port 80) redirects to HTTPS (port 443)
3. **API Proxy**: `/api/*` paths are proxied to the nosd service
4. **SPA Support**: All other paths serve the web UI with fallback to `index.html`
5. **Security Headers**: Includes standard security headers for the HTTPS site

### Configuration Content

```caddyfile
# nOS managed Caddyfile
{
  admin localhost:2019
}

http://:80 {
  @api path /api/*
  reverse_proxy @api 127.0.0.1:9000
  redir https://{host}{uri} 308
}

https://:443 {
  tls internal

  encode gzip zstd
  header {
    X-Content-Type-Options "nosniff"
    Referrer-Policy "no-referrer"
    Cross-Origin-Opener-Policy "same-origin"
    Cross-Origin-Embedder-Policy "require-corp"
  }

  @api path /api/*
  reverse_proxy @api 127.0.0.1:9000

  handle {
    root * /usr/share/nos/web
    try_files {path} /index.html
    file_server
  }
}
```

## Installation Flow

### Package Installation (nos-web)

The `nos-web` package postinst script:
1. Creates necessary directories
2. Writes the managed Caddyfile (if missing or already managed)
3. Creates systemd override for service ordering
4. Validates the configuration
5. Enables and starts Caddy

### ISO Build

During ISO creation:
1. Caddyfile is placed at `/etc/caddy/Caddyfile` via includes.chroot
2. Systemd override is placed at `/etc/systemd/system/caddy.service.d/override.conf`
3. Hook `040-enable-caddy.hook.chroot` enables the service

### Systemd Service Ordering

The override configuration ensures:
- Caddy starts after network is online
- Caddy starts after nosd service
- Configuration is validated before starting
- Service automatically restarts on failure

```ini
[Unit]
Wants=network-online.target
After=network-online.target nosd.service

[Service]
ExecStartPre=/usr/bin/caddy validate --config /etc/caddy/Caddyfile
Restart=always
RestartSec=2s
```

## Verification

Use the included verification script:
```bash
scripts/dev/check-caddy.sh
```

This script checks:
1. Configuration validity
2. Listening sockets (80, 443, 9000)
3. HTTP to HTTPS redirect
4. API endpoint accessibility

Manual verification:
```bash
# Check config
caddy validate --config /etc/caddy/Caddyfile

# Check service status
systemctl status caddy

# Test HTTP redirect
curl -I http://127.0.0.1/

# Test HTTPS (self-signed cert)
curl -sk https://127.0.0.1/api/setup/state
```

## Troubleshooting

### Caddy Not Starting
- Check logs: `journalctl -u caddy -n 50`
- Validate config: `caddy validate --config /etc/caddy/Caddyfile`
- Ensure nosd is running: `systemctl status nosd`

### HTTPS Certificate Issues
- Caddy's `tls internal` creates self-signed certificates automatically
- Certificates are stored in Caddy's data directory
- Browser warnings are expected for self-signed certificates

### Port Conflicts
- Ensure no other services are using ports 80 or 443
- Check with: `ss -ltnp | grep -E ':80|:443'`

## Security Notes

- The configuration uses `tls internal` for automatic self-signed certificates
- This is suitable for local/private networks and development
- For production with public access, consider using Let's Encrypt or custom certificates
- Security headers are applied to all HTTPS responses

## Maintenance

### Updating Configuration
1. Edit `/etc/caddy/Caddyfile`
2. Validate: `caddy validate --config /etc/caddy/Caddyfile`
3. Reload: `systemctl reload caddy`

### Checking Logs
```bash
# Service logs
journalctl -u caddy -f

# Access logs (if configured)
tail -f /var/log/caddy/access.log
```
