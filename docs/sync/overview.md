# NithronSync Overview

NithronSync is NithronOS's built-in file synchronization system, providing OneDrive-like functionality for your self-hosted storage. It enables seamless file sync across Windows, Linux, macOS desktops, and iOS/Android mobile devices.

## Key Features

- **Cross-Platform Sync** — Windows, Linux, macOS, iOS, and Android client support
- **Delta Sync** — Only changed portions of files are transferred, minimizing bandwidth
- **Device Management** — Register, monitor, and revoke sync devices from the web UI
- **WebDAV Access** — Standard protocol for broad client compatibility
- **Offline Support** — Work offline, sync when reconnected
- **Version Control** — Optional file versioning with configurable retention
- **Selective Sync** — Choose which folders to sync on each device
- **Bandwidth Control** — Limit sync speeds to preserve network capacity

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        NithronOS Server                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │  Sync API   │  │   WebDAV    │  │    Change Tracker       │ │
│  │  /api/v1/   │  │   /dav/     │  │  (fsnotify + cursors)   │ │
│  │   sync/*    │  │  {share}/   │  │                         │ │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘ │
│         │                │                      │               │
│  ┌──────┴────────────────┴──────────────────────┴─────────────┐ │
│  │                    Device Manager                           │ │
│  │  • Token generation & validation                           │ │
│  │  • Device registration & lifecycle                         │ │
│  │  • Scope-based permissions                                 │ │
│  └─────────────────────────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    Delta Sync Engine                        │ │
│  │  • Block-based hashing (SHA-256 + Adler-32)                │ │
│  │  • Rolling checksum for efficient diff                     │ │
│  │  • Transfer plan generation                                │ │
│  └─────────────────────────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                    Storage Layer                            │ │
│  │  • Sync-enabled shares                                     │ │
│  │  • Device & configuration storage                          │ │
│  │  • Sync state per device/share                             │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ HTTPS
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Sync Clients                              │
├───────────┬───────────┬───────────┬───────────┬─────────────────┤
│  Windows  │   Linux   │   macOS   │    iOS    │    Android      │
│  Desktop  │  Desktop  │  Desktop  │   App     │     App         │
└───────────┴───────────┴───────────┴───────────┴─────────────────┘
```

## Quick Start

### 1. Enable Sync on a Share

1. Go to **Shares** in the web UI
2. Create or edit a share
3. Navigate to the **Sync** tab
4. Toggle **Enable NithronSync**
5. Configure optional settings (max size, exclude patterns)
6. Save

### 2. Register a Device

1. Go to **Settings → Sync → Devices**
2. Click **Register Device**
3. Enter a device name and select the type
4. Copy the generated token or scan the QR code
5. Enter the token in your sync client

### 3. Connect Client

Download the appropriate client for your platform:

| Platform | Download | Notes |
|----------|----------|-------|
| Windows | [NithronSync for Windows](#) | Windows 10/11, 64-bit |
| Linux | [NithronSync for Linux](#) | AppImage or .deb |
| macOS | [NithronSync for macOS](#) | macOS 12+ (Intel/ARM) |
| iOS | [App Store](#) | iOS 15+ |
| Android | [Play Store](#) | Android 10+ |

## Security Model

### Device Tokens

Each registered device receives:
- **Access Token** — Short-lived (24h), used for API requests
- **Refresh Token** — Long-lived (90 days), used to obtain new access tokens

Tokens are:
- Generated with cryptographic randomness
- Stored hashed (bcrypt) on the server
- Scoped to specific permissions (`sync.read`, `sync.write`)
- Revocable from the web UI

### API Scopes

| Scope | Description |
|-------|-------------|
| `sync.read` | Read files via sync API |
| `sync.write` | Write files via sync API |
| `sync.devices` | Manage own devices |
| `sync.admin` | Full sync administration |

### Network Security

- All sync traffic uses HTTPS (TLS 1.3)
- Device tokens are transmitted via `Authorization: Bearer` header
- WebDAV requests require device token authentication
- Rate limiting protects against brute-force attacks

## API Endpoints

All sync endpoints are prefixed with `/api/v1/sync`:

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/devices/register` | Register new device |
| POST | `/devices/refresh` | Refresh access token |
| GET | `/devices` | List registered devices |
| DELETE | `/devices/{id}` | Revoke device |
| GET | `/shares` | List sync-enabled shares |
| GET | `/config` | Get device sync configuration |
| PUT | `/config` | Update sync configuration |
| GET | `/changes` | Get file changes (delta) |
| GET | `/files/{share}/metadata` | Get file/folder metadata |
| POST | `/files/{share}/hash` | Get block hashes for delta sync |
| GET | `/state/{share}` | Get sync state for share |
| PUT | `/state/{share}` | Update sync state |

WebDAV access is available at `/dav/{share_id}/`.

## Related Documentation

- [API Reference](./api.md) — Detailed API documentation
- [Client Setup](./client-setup.md) — Platform-specific client installation
- [Troubleshooting](./troubleshooting.md) — Common issues and solutions
- [Security](./security.md) — Security considerations and best practices

## Roadmap

### Phase 1 ✅
- Server-side sync infrastructure
- Device management and authentication
- WebDAV file access
- Web UI for devices and settings
- Delta sync algorithm

### Phase 2 ✅
- Desktop client for Windows/Linux/macOS
- Background sync daemon
- System tray integration
- Selective sync folders
- Platform-specific installers

### Phase 3 ✅
- Mobile apps for iOS/Android
- Conflict resolution UI
- Shared folder collaboration
- Sync activity history
- App store deployment automation

### Phase 4 ✅
- Real-time collaboration (WebSocket, presence, live cursors)
- End-to-end encryption (AES-GCM, ChaCha20-Poly1305, key management)
- Offline-first sync (operation queue, conflict resolution)
- Smart sync (on-demand files, placeholders, hydration)

