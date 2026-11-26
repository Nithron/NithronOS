# nOS Certificate Configuration Guide

## Overview

nOS supports multiple certificate strategies depending on your deployment scenario:

1. **Self-Signed Certificates** (Default) - For development and internal networks
2. **Let's Encrypt** - For production with public domain
3. **Local CA** - For enterprise environments
4. **Custom Certificates** - Bring your own certificates

## Current Setup

By default, nOS uses Caddy's `tls internal` directive which:
- Automatically generates self-signed certificates
- Works with IP addresses
- Requires accepting certificate warning in browser
- Perfect for development and testing

## Certificate Options

### Option 1: Local Domain with Self-Signed (Recommended for Home/Lab)

This approach uses a local domain name that works on your network:

```bash
# Run on nOS:
sudo /usr/local/bin/nos-setup-local-domain.sh
```

This sets up:
- Local domain: `nithron.local`
- mDNS/Avahi for automatic discovery
- Access via: `https://nithron.local`

### Option 2: Dynamic DNS with Let's Encrypt (For Remote Access)

If you need external access with valid certificates:

1. **Set up Dynamic DNS** (DuckDNS, No-IP, etc.):
   ```bash
   # Example with DuckDNS
   yourdomain.duckdns.org → Your public IP
   ```

2. **Configure port forwarding** on your router:
   - Forward ports 80 and 443 to nOS

3. **Update Caddyfile**:
   ```caddyfile
   yourdomain.duckdns.org {
     # Caddy will automatically get Let's Encrypt cert
     
     @api path /api/*
     handle @api {
       reverse_proxy 127.0.0.1:9000
     }
     
     handle {
       root * /usr/share/nos/web
       try_files {path} /index.html
       file_server
     }
   }
   ```

### Option 3: Local DNS Server (Like Fritz!Box)

Similar to your Fritz!Box setup, you can:

1. **Use router's local DNS**:
   - Add DNS entry: `nithron.home → nOS-IP`
   - Access via: `https://nithron.home`

2. **Use Pi-hole or similar**:
   - Add local DNS record
   - Automatic for all devices on network

### Option 4: Split-Horizon DNS (Best for Mixed Access)

For both local and remote access:

```caddyfile
# In Caddyfile
nithron.local, yourdomain.duckdns.org {
  # Works for both local and external access
  tls {
    # Let's Encrypt for public domain
    # Falls back to self-signed for .local
  }
  
  # ... rest of config
}
```

## Wildcard Certificates

For multiple services:

```caddyfile
*.nithron.local {
  tls internal
  
  # Handles:
  # - nithron.local
  # - app.nithron.local
  # - api.nithron.local
  # etc.
}
```

## Custom Certificate Installation

If you have your own certificates:

```bash
# Place certificates in:
/etc/caddy/certs/

# Update Caddyfile:
nithron.local {
  tls /etc/caddy/certs/cert.pem /etc/caddy/certs/key.pem
  # ... rest of config
}
```

## Automatic Certificate on Boot

To handle changing IPs, add to startup:

```bash
# In /etc/systemd/system/nos-update-domain.service
[Unit]
Description=Update nOS Domain on IP Change
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/nos-setup-local-domain.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
```

## Testing Certificate Configuration

```bash
# Check Caddy configuration
caddy validate --config /etc/caddy/Caddyfile

# Test HTTPS
curl -k https://localhost

# Check certificate
openssl s_client -connect localhost:443 -servername nithron.local
```

## Troubleshooting

### Browser Still Shows Certificate Warning
- **Expected** for self-signed certificates
- Add permanent exception in browser
- Or use Let's Encrypt with public domain

### Let's Encrypt Rate Limits
- 50 certificates per domain per week
- Use staging environment for testing:
  ```caddyfile
  tls {
    ca https://acme-staging-v02.api.letsencrypt.org/directory
  }
  ```

### Certificate Not Updating
```bash
# Force certificate renewal
systemctl stop caddy
rm -rf /var/lib/caddy/.local/share/caddy
systemctl start caddy
```

## Security Considerations

1. **Self-Signed**: Fine for internal use, browsers will warn
2. **Let's Encrypt**: Best for public-facing, requires domain
3. **Local CA**: Best for enterprise, requires PKI infrastructure

## Recommended Setup by Use Case

| Use Case | Recommended Solution |
|----------|---------------------|
| Home Lab | Local domain with self-signed |
| Development | IP-based with self-signed |
| Small Business | Dynamic DNS + Let's Encrypt |
| Enterprise | Internal CA with custom certs |
| Public Service | Real domain + Let's Encrypt |

## Quick Start Commands

```bash
# For local network access (recommended)
sudo /usr/local/bin/nos-setup-local-domain.sh

# Then access at:
https://nithron.local

# For public access with Let's Encrypt
# 1. Get a domain (or use dynamic DNS)
# 2. Update /etc/caddy/Caddyfile with your domain
# 3. Restart Caddy:
sudo systemctl restart caddy
```
