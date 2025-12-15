# NithronSync Security

This document describes the security model and best practices for NithronSync.

## Overview

NithronSync is designed with security as a priority, following the same principles as NithronOS:
- **Local-first** — Your data stays on your server
- **Zero trust** — All connections are authenticated and encrypted
- **Minimal attack surface** — Only necessary APIs are exposed

## Authentication

### Device Token System

NithronSync uses a device token system separate from user sessions:

```
┌─────────────┐     ┌─────────────────────────────────────────────┐
│   Client    │     │              NithronOS Server               │
│   Device    │     │                                             │
│             │     │  ┌─────────────┐    ┌─────────────────────┐ │
│  ┌───────┐  │     │  │   Device    │    │     User Auth       │ │
│  │Device │──┼─────┼─▶│   Manager   │◀───│    (Sessions)       │ │
│  │Token  │  │     │  │             │    │                     │ │
│  └───────┘  │     │  └──────┬──────┘    └─────────────────────┘ │
│             │     │         │                                   │
│  ┌───────┐  │     │  ┌──────▼──────┐                           │
│  │Access │──┼─────┼─▶│   Sync API  │                           │
│  │Token  │  │     │  │             │                           │
│  └───────┘  │     │  └─────────────┘                           │
└─────────────┘     └─────────────────────────────────────────────┘
```

### Token Types

| Token | Storage | Lifetime | Purpose |
|-------|---------|----------|---------|
| Device Token | Hashed in DB | Permanent | Device identity |
| Access Token | Not stored | 24 hours | API authentication |
| Refresh Token | Hashed in DB | 90 days | Token renewal |

### Token Generation

Tokens are generated using cryptographically secure random bytes:

```go
// 32 bytes of entropy, base64url encoded
token := "nos_at_" + base64.URLEncoding.EncodeToString(randomBytes(32))
```

### Token Validation

1. Token format is verified (prefix, length)
2. Token is hashed and looked up in database
3. Expiration is checked
4. Associated device status is verified
5. Rate limits are enforced

### Token Revocation

When a device is revoked:
- All associated tokens are immediately invalidated
- Active connections are terminated
- Device is removed from device list

## Authorization

### API Scopes

Device tokens are granted specific scopes:

| Scope | Permissions |
|-------|-------------|
| `sync.read` | Read files, list directories, get metadata |
| `sync.write` | Upload files, create directories, delete files |
| `sync.devices` | View own devices, update device name |
| `sync.admin` | Manage all devices (admin only) |

### Share Access Control

Access to shares is controlled by:

1. **Share ownership** — User must own or have access to the share
2. **Sync enabled** — Share must have sync explicitly enabled
3. **Allowed users** — Optional per-share user allowlist
4. **Path restrictions** — Sync is limited to share boundaries

```json
{
  "share_id": "share_123",
  "sync_enabled": true,
  "sync_allowed_users": ["user_abc", "user_def"]
}
```

## Encryption

### In Transit

All sync traffic is encrypted using TLS 1.3:
- HTTPS for API and WebDAV
- Certificate pinning available in clients
- Self-signed certificates supported (with explicit trust)

### At Rest

Files on the server use NithronOS storage encryption:
- LUKS2 for full-disk encryption
- Per-pool encryption options
- Key management via NithronOS

### Client-Side Encryption (Optional)

For sensitive data, clients support end-to-end encryption:
- Files encrypted before upload
- Keys never leave the client
- Server stores encrypted blobs only
- Available in client settings → Security

## Rate Limiting

API endpoints are rate-limited to prevent abuse:

| Endpoint | Limit | Window |
|----------|-------|--------|
| Device registration | 10 | Per hour |
| Token refresh | 60 | Per hour |
| File changes | 120 | Per minute |
| File operations | 1000 | Per hour |
| Failed auth attempts | 5 | Per 15 min |

### Lockout Policy

After 5 failed authentication attempts:
- Device is locked for 15 minutes
- User is notified via email (if configured)
- Event is logged for audit

## Audit Logging

All sync activities are logged:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "event": "file.upload",
  "device_id": "dt_abc123",
  "user_id": "usr_xyz",
  "share_id": "share_123",
  "path": "/Documents/report.pdf",
  "ip_address": "192.168.1.100",
  "user_agent": "NithronSync/1.0.0 Windows"
}
```

Log events include:
- Device registration/revocation
- Authentication success/failure
- File operations (create, modify, delete)
- Configuration changes
- Errors and anomalies

## Network Security

### Firewall Recommendations

NithronSync uses these ports:
- **443 (HTTPS)** — API and WebDAV

Recommended firewall rules:
```
# Allow NithronSync from LAN only
iptables -A INPUT -p tcp --dport 443 -s 192.168.0.0/16 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -j DROP
```

### Remote Access

For internet access, use one of:
1. **VPN** — WireGuard tunnel to your network (recommended)
2. **Reverse proxy** — Cloudflare Tunnel, ngrok, etc.
3. **Direct exposure** — Ensure 2FA and strong passwords

See [Networking Guide](../networking.md) for detailed remote access setup.

## Security Best Practices

### For Users

1. **Use strong passwords** — Follow NithronOS password requirements
2. **Enable 2FA** — Protects device registration
3. **Review devices regularly** — Revoke unused devices
4. **Use selective sync** — Only sync what you need
5. **Keep clients updated** — Security fixes in updates

### For Administrators

1. **Enable HTTPS** — Required for production use
2. **Use valid certificates** — Let's Encrypt or trusted CA
3. **Limit sync shares** — Only enable sync on needed shares
4. **Monitor audit logs** — Watch for suspicious activity
5. **Set device limits** — Prevent excessive registrations
6. **Regular backups** — Protect against data loss

### For Developers

1. **Validate all input** — Prevent injection attacks
2. **Use parameterized queries** — No SQL injection
3. **Sanitize paths** — Prevent directory traversal
4. **Rate limit everything** — Prevent DoS
5. **Log security events** — Enable incident response

## Threat Model

### Threats Mitigated

| Threat | Mitigation |
|--------|------------|
| Token theft | Short-lived access tokens, token rotation |
| Brute force | Rate limiting, lockout policy |
| Man-in-the-middle | TLS encryption, certificate pinning |
| Replay attacks | Token expiration, nonces |
| Unauthorized access | Scope-based authorization |
| Data exposure | Encryption at rest and in transit |

### Out of Scope

These threats require additional measures:
- **Physical device theft** — Enable device encryption, remote wipe
- **Compromised client** — Use endpoint security software
- **Server compromise** — Follow NithronOS security guide
- **Social engineering** — User security awareness training

## Security Contacts

- **Security issues:** security@nithron.com
- **Responsible disclosure:** See [SECURITY.md](../../SECURITY.md)
- **Bug bounty:** Contact us for details

## Compliance

NithronSync is designed to support compliance requirements:
- **Data locality** — All data stays on your server
- **Access control** — Role-based permissions
- **Audit trail** — Comprehensive logging
- **Encryption** — TLS 1.3 + optional E2E

For specific compliance needs (GDPR, HIPAA, etc.), consult with your compliance team and configure NithronSync accordingly.

