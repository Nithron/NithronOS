# NithronSync Security

This document describes the security model and best practices for NithronSync.

## Overview

NithronSync is designed with security as a priority, following the same principles as nOS:
- **Local-first** — Your data stays on your server
- **Zero trust** — All connections are authenticated and encrypted
- **Minimal attack surface** — Only necessary APIs are exposed

## Authentication

### Device Token System

NithronSync uses a device token system separate from user sessions:

```
┌─────────────┐     ┌─────────────────────────────────────────────┐
│   Client    │     │              nOS Server               │
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

| Token | Prefix | Storage | Lifetime | Purpose |
|-------|--------|---------|----------|---------|
| Device token | `nos_dt_` | bcrypt hash | 90 days | Every API and WebDAV request |
| Refresh token | `nos_rt_` | bcrypt hash | 365 days | A new device token (both are replaced) |

There is no separate access token: the device token is sent with every
request (`Authorization: Bearer`, or as the Basic password for WebDAV).

### Token Generation

32 random bytes from `crypto/rand`, base64url encoded, with the prefix
(`backend/nosd/pkg/sync/device_manager.go`). Both tokens are shown once, in
the pairing link.

### Server identity

nOS's web certificate is self-signed, so a client can't check it against
certificate authorities. The pairing link carries the SHA-256 of the
certificate's public key and the client accepts only that key
(`clients/sync-core/api/tls.go`). The key survives certificate renewal; a
reinstalled machine has a new one, and devices must be paired again.

### Token Validation

1. The prefix is checked
2. The token is compared with the stored bcrypt hashes
3. Expiry and revocation are checked

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

Files on the server use nOS storage encryption:
- LUKS2 for full-disk encryption
- Per-pool encryption options
- Key management via nOS

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

1. **Use strong passwords** — Follow nOS password requirements
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
- **Server compromise** — Follow nOS security guide
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

