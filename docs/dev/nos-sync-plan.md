# NithronSync - File Sync Application Plan

## Executive Summary

NithronSync is a cross-platform file synchronization application that works like OneDrive/Dropbox, enabling seamless file access and synchronization between NithronOS NAS servers and client devices (Windows, Linux, Android, iOS). This document outlines the requirements, specifications, backend changes, and implementation plan.

---

## Table of Contents

1. [Product Vision](#product-vision)
2. [Requirements](#requirements)
3. [Technical Specifications](#technical-specifications)
4. [Backend Changes to NithronOS](#backend-changes-to-nithronos)
5. [Client Applications](#client-applications)
6. [Security Model](#security-model)
7. [Implementation Plan](#implementation-plan)
8. [Future Enhancements](#future-enhancements)

---

## Product Vision

### Goals

1. **Seamless File Access**: Files stored on NithronOS are accessible from any device
2. **Automatic Sync**: Changes made on any device automatically propagate to all others
3. **Offline Support**: Work with files offline; changes sync when reconnected
4. **Cross-Platform**: Native apps for Windows, Linux, Android, and iOS
5. **Privacy-First**: All data stays on user's NithronOS server; no cloud intermediaries
6. **Integration**: Extends existing NithronOS shares infrastructure

### Non-Goals (v1)

- End-to-end encryption (server already has access to files)
- Peer-to-peer sync between devices without NithronOS
- Real-time collaborative editing
- File versioning in mobile apps (rely on server snapshots)

---

## Requirements

### Functional Requirements

#### FR1: User Authentication & Device Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR1.1 | Users authenticate with NithronOS credentials | P0 |
| FR1.2 | Support 2FA (TOTP) during device registration | P0 |
| FR1.3 | Each device gets a unique device token | P0 |
| FR1.4 | Device tokens can be revoked from web UI | P0 |
| FR1.5 | Device list viewable in web UI with last sync time | P1 |
| FR1.6 | Automatic token refresh without user intervention | P0 |

#### FR2: File Synchronization

| ID | Requirement | Priority |
|----|-------------|----------|
| FR2.1 | Two-way sync between device and NithronOS | P0 |
| FR2.2 | Selective sync (choose folders/shares to sync) | P0 |
| FR2.3 | Conflict detection and resolution | P0 |
| FR2.4 | Delta sync (only transfer changed blocks) | P1 |
| FR2.5 | Bandwidth throttling options | P1 |
| FR2.6 | Pause/resume sync operations | P0 |
| FR2.7 | Real-time sync (sub-minute) on LAN | P1 |
| FR2.8 | Periodic sync when on mobile data | P2 |

#### FR3: Desktop Integration (Windows/Linux)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR3.1 | Virtual folder in file explorer (like OneDrive) | P0 |
| FR3.2 | Files On-Demand (placeholder files) | P1 |
| FR3.3 | Right-click context menu integration | P0 |
| FR3.4 | System tray icon with sync status | P0 |
| FR3.5 | Desktop notifications for sync events | P1 |
| FR3.6 | Auto-start on system boot | P0 |
| FR3.7 | Pin/unpin files for offline access | P1 |

#### FR4: Mobile Integration (Android/iOS)

| ID | Requirement | Priority |
|----|-------------|----------|
| FR4.1 | Browse all synced files | P0 |
| FR4.2 | Download files on-demand | P0 |
| FR4.3 | Upload files/photos from device | P0 |
| FR4.4 | Automatic camera roll backup | P1 |
| FR4.5 | Offline file access (marked favorites) | P0 |
| FR4.6 | Share files via system share sheet | P1 |
| FR4.7 | Background sync when on WiFi | P1 |

#### FR5: Server-Side Features

| ID | Requirement | Priority |
|----|-------------|----------|
| FR5.1 | Expose sync-enabled shares via new API | P0 |
| FR5.2 | File change tracking (modification times, hashes) | P0 |
| FR5.3 | WebDAV endpoint for file operations | P0 |
| FR5.4 | Efficient file listing with pagination | P0 |
| FR5.5 | File metadata API (size, mtime, hash) | P0 |
| FR5.6 | Chunked uploads/downloads for large files | P0 |
| FR5.7 | Resume interrupted transfers | P1 |

### Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NFR1 | Sync latency on LAN | < 30 seconds |
| NFR2 | Initial sync of 10GB folder | < 10 minutes (1Gbps) |
| NFR3 | Desktop client memory usage | < 200MB |
| NFR4 | Mobile app battery impact | < 3% daily (background) |
| NFR5 | Concurrent device support | 10+ devices per user |
| NFR6 | Maximum file size | 50GB |
| NFR7 | API availability | 99.9% (limited by hardware) |

---

## Technical Specifications

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         NithronOS Server                         │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   nosd      │  │ nos-agent   │  │     Caddy Proxy         │  │
│  │  (Go API)   │  │ (privileged)│  │  (HTTPS/WebDAV/WS)      │  │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘  │
│         │                │                      │                │
│         ▼                ▼                      │                │
│  ┌─────────────────────────────────────────────┴───────────────┐│
│  │                    Sync Engine (new)                         ││
│  │  ┌─────────┐ ┌──────────┐ ┌─────────┐ ┌────────────────┐    ││
│  │  │ Change  │ │ WebDAV   │ │ Delta   │ │ Device Token   │    ││
│  │  │ Tracker │ │ Handler  │ │ Sync    │ │ Manager        │    ││
│  │  └─────────┘ └──────────┘ └─────────┘ └────────────────┘    ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│                              ▼                                   │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    Btrfs Filesystem                          ││
│  │    /srv/shares/<share>/  (existing share infrastructure)    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                               │
            ┌──────────────────┼──────────────────┐
            │                  │                  │
            ▼                  ▼                  ▼
    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │   Desktop    │  │   Android    │  │     iOS      │
    │   Client     │  │     App      │  │     App      │
    │ (Win/Linux)  │  │              │  │              │
    └──────────────┘  └──────────────┘  └──────────────┘
```

### Sync Protocol Design

#### 1. Change Detection (Server-Side)

**Option A: Polling-based (Recommended for v1)**
- Client polls `/api/v1/sync/changes` endpoint
- Server compares file mtimes against last sync cursor
- Returns list of changed files with metadata

**Option B: Event-based (v2 Enhancement)**
- inotify/fanotify watches on sync-enabled shares
- WebSocket push to connected clients
- More efficient but complex

#### 2. Sync State Machine

```
     ┌────────────────────────────────────────────────────┐
     │                                                    │
     ▼                                                    │
┌─────────┐    ┌──────────┐    ┌──────────┐    ┌────────┴──┐
│  IDLE   │───▶│ SCANNING │───▶│ SYNCING  │───▶│ UPLOADING │
└─────────┘    └──────────┘    └──────────┘    └───────────┘
     ▲              │               │                │
     │              ▼               ▼                ▼
     │         ┌──────────┐   ┌──────────┐    ┌───────────┐
     └─────────│  ERROR   │   │DOWNLOAD- │    │ COMPLETED │
               └──────────┘   │   ING    │    └───────────┘
                              └──────────┘
```

#### 3. Conflict Resolution Strategy

| Scenario | Resolution |
|----------|------------|
| Same file modified on client and server | Keep both: `file.txt` + `file (conflict-<device>-<date>).txt` |
| File deleted on server, modified on client | Re-upload client version |
| File deleted on client, modified on server | Re-download server version |
| Same file deleted on both | Delete confirmed |
| Directory conflicts | Merge contents |

#### 4. Delta Sync Algorithm

For files > 1MB:
1. Compute rolling checksums (rsync-style) on both ends
2. Transfer only different blocks
3. Reconstruct file on destination
4. Verify full file hash after assembly

Use [zsync](http://zsync.moria.org.uk/) or similar algorithm for HTTP-compatible delta sync.

### API Endpoints (New)

#### Device Management

```yaml
POST /api/v1/sync/devices/register
  # Register a new sync device
  Request:
    device_name: string
    device_type: "windows" | "linux" | "android" | "ios"
    os_version: string
    client_version: string
  Response:
    device_id: string
    device_token: string  # Long-lived token for this device
    refresh_token: string

POST /api/v1/sync/devices/refresh
  # Refresh device token (uses refresh_token)
  
GET /api/v1/sync/devices
  # List all registered devices for user
  
DELETE /api/v1/sync/devices/{device_id}
  # Revoke device access
```

#### Sync Configuration

```yaml
GET /api/v1/sync/shares
  # List sync-enabled shares accessible to user
  Response:
    shares:
      - id: string
        name: string
        path: string
        total_size: number
        file_count: number
        sync_enabled: boolean

PUT /api/v1/sync/config
  # Set sync preferences for device
  Request:
    device_id: string
    sync_shares: string[]  # Share IDs to sync
    selective_paths: string[]  # Optional: specific paths within shares
    bandwidth_limit_kbps: number
```

#### File Operations

```yaml
GET /api/v1/sync/changes
  # Get changes since last sync
  Query:
    since_cursor: string  # Opaque cursor from last sync
    share_id: string
    limit: number (default: 1000)
  Response:
    changes:
      - path: string
        type: "create" | "modify" | "delete" | "rename"
        size: number
        mtime: string (ISO8601)
        hash: string (SHA-256)
        old_path: string (for renames)
    cursor: string  # For next request
    has_more: boolean

GET /api/v1/sync/files/{share_id}/metadata
  # Get metadata for files
  Query:
    paths: string[]  # File paths
  Response:
    files:
      - path: string
        size: number
        mtime: string
        hash: string
        is_dir: boolean

POST /api/v1/sync/files/{share_id}/hash
  # Get block hashes for delta sync
  Request:
    path: string
    block_size: number (default: 4MB)
  Response:
    blocks:
      - offset: number
        size: number
        hash: string
```

#### WebDAV Endpoint

```
/dav/{share_id}/
  # Standard WebDAV for file operations
  # Mounted at Caddy level, proxied to nosd
  # Supports: GET, PUT, DELETE, MKCOL, MOVE, COPY, PROPFIND
```

### Data Models

#### Device Token

```go
type DeviceToken struct {
    ID           string    `json:"id"`
    UserID       string    `json:"user_id"`
    DeviceName   string    `json:"device_name"`
    DeviceType   string    `json:"device_type"`  // windows, linux, android, ios
    OSVersion    string    `json:"os_version"`
    ClientVersion string   `json:"client_version"`
    TokenHash    string    `json:"-"`  // bcrypt hash
    RefreshHash  string    `json:"-"`
    CreatedAt    time.Time `json:"created_at"`
    LastSyncAt   *time.Time `json:"last_sync_at"`
    LastIP       string    `json:"last_ip"`
    Scopes       []string  `json:"scopes"`  // e.g., ["sync.read", "sync.write"]
    RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}
```

#### Sync State (per device)

```go
type SyncState struct {
    DeviceID    string            `json:"device_id"`
    ShareID     string            `json:"share_id"`
    Cursor      string            `json:"cursor"`  // Opaque sync position
    LastSync    time.Time         `json:"last_sync"`
    Files       map[string]FileState `json:"files"`
}

type FileState struct {
    Path     string    `json:"path"`
    Size     int64     `json:"size"`
    MTime    time.Time `json:"mtime"`
    Hash     string    `json:"hash"`  // SHA-256
    SyncedAt time.Time `json:"synced_at"`
}
```

#### Sync-Enabled Share

```go
type SyncShare struct {
    ShareID       string   `json:"share_id"`
    SyncEnabled   bool     `json:"sync_enabled"`
    AllowedUsers  []string `json:"allowed_users"`
    MaxSyncSize   int64    `json:"max_sync_size"`  // Optional limit
    ExcludePatterns []string `json:"exclude_patterns"`  // e.g., ["*.tmp", ".git"]
}
```

---

## Backend Changes to NithronOS

### 1. New Package: `pkg/sync`

Create a new package for sync-related functionality:

```
backend/nosd/pkg/sync/
├── device_manager.go      # Device registration & token management
├── device_manager_test.go
├── change_tracker.go      # File change detection
├── change_tracker_test.go
├── delta.go               # Delta sync algorithm
├── delta_test.go
├── webdav.go              # WebDAV handler integration
├── webdav_test.go
├── types.go               # Data structures
└── store.go               # Persistence (sync state, devices)
```

### 2. Extend Existing Components

#### 2.1 Auth Package (`pkg/auth`)

**File**: `backend/nosd/pkg/auth/token.go`

Add new token scopes for sync:

```go
const (
    // Existing scopes...
    
    // New sync scopes
    ScopeSyncRead    TokenScope = "sync.read"   // Read files via sync API
    ScopeSyncWrite   TokenScope = "sync.write"  // Write files via sync API
    ScopeSyncDevices TokenScope = "sync.devices" // Manage sync devices
)

// New token type for sync devices
const TokenTypeDevice TokenType = "device"
```

Add device token creation method:

```go
func (tm *TokenManager) CreateDeviceToken(req CreateDeviceTokenRequest) (*DeviceToken, string, string, error) {
    // Similar to CreateToken but:
    // - Uses "nos_dt_" prefix
    // - Creates refresh token too
    // - Stores device metadata
}
```

#### 2.2 Shares Package (`internal/shares`)

**File**: `backend/nosd/internal/shares/store.go`

Extend Share struct:

```go
type Share struct {
    // Existing fields...
    
    // New sync fields
    SyncEnabled     bool     `json:"sync_enabled"`
    SyncMaxSize     int64    `json:"sync_max_size,omitempty"`
    SyncExclude     []string `json:"sync_exclude,omitempty"`
}
```

#### 2.3 Server Routes

**File**: `backend/nosd/internal/server/router.go`

Add new route group:

```go
// Sync API routes (requires device token auth)
r.Route("/api/v1/sync", func(r chi.Router) {
    r.Use(DeviceTokenAuthMiddleware)
    
    // Device management
    r.Post("/devices/register", h.RegisterDevice)
    r.Post("/devices/refresh", h.RefreshDeviceToken)
    r.Get("/devices", h.ListDevices)
    r.Delete("/devices/{device_id}", h.RevokeDevice)
    
    // Sync configuration
    r.Get("/shares", h.ListSyncShares)
    r.Put("/config", h.UpdateSyncConfig)
    
    // File operations
    r.Get("/changes", h.GetChanges)
    r.Get("/files/{share_id}/metadata", h.GetFileMetadata)
    r.Post("/files/{share_id}/hash", h.GetBlockHashes)
})

// WebDAV endpoint (separate auth middleware)
r.Mount("/dav", webdav.Handler(h.webdavHandler))
```

### 3. New Handler: `sync_handler.go`

**File**: `backend/nosd/internal/server/sync_handler.go`

```go
type SyncHandler struct {
    deviceMgr     *sync.DeviceManager
    changeTracker *sync.ChangeTracker
    shareStore    *shares.Store
    webdavHandler *webdav.Handler
    logger        zerolog.Logger
}

func (h *SyncHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
    // 1. Validate user authentication (initial registration uses user token)
    // 2. Verify 2FA if enabled
    // 3. Create device token
    // 4. Return tokens
}

func (h *SyncHandler) GetChanges(w http.ResponseWriter, r *http.Request) {
    // 1. Parse cursor from query
    // 2. Get device from context
    // 3. Query change tracker for files changed since cursor
    // 4. Return changes with new cursor
}
```

### 4. WebDAV Integration

**File**: `backend/nosd/internal/server/webdav_handler.go`

Integrate with Go's `golang.org/x/net/webdav`:

```go
import "golang.org/x/net/webdav"

type NosWebDAV struct {
    shareStore *shares.Store
    authMgr    *auth.TokenManager
    logger     zerolog.Logger
}

func (n *NosWebDAV) Handler() http.Handler {
    return &webdav.Handler{
        Prefix:     "/dav",
        FileSystem: n, // Implement webdav.FileSystem
        LockSystem: webdav.NewMemLS(),
        Logger:     n.logRequest,
    }
}

// Implement webdav.FileSystem interface
func (n *NosWebDAV) Mkdir(ctx context.Context, name string, perm os.FileMode) error
func (n *NosWebDAV) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error)
func (n *NosWebDAV) RemoveAll(ctx context.Context, name string) error
func (n *NosWebDAV) Rename(ctx context.Context, oldName, newName string) error
func (n *NosWebDAV) Stat(ctx context.Context, name string) (os.FileInfo, error)
```

### 5. Caddy Configuration Updates

**File**: `packaging/deb/caddy-nithronos/Caddyfile`

Add WebDAV routing:

```caddyfile
# WebDAV for sync clients
handle /dav/* {
    reverse_proxy 127.0.0.1:9000 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        
        # Longer timeouts for file transfers
        transport http {
            dial_timeout 30s
            response_header_timeout 600s  # 10 min for large files
        }
    }
}
```

### 6. Agent Extensions

**File**: `agent/nos-agent/internal/server/sync_handler.go`

Add agent support for file operations requiring elevated privileges:

```go
// Only needed if sync operations require root (e.g., preserving ownership)
func handleSyncFileOp(w http.ResponseWriter, r *http.Request) {
    // Validate request via allowlist
    // Perform chown/chmod if needed
}
```

### 7. Database/State Storage

**Files to create/modify**:

- `/etc/nos/sync/devices.json` - Registered devices
- `/etc/nos/sync/state/` - Per-device sync state
- `/var/lib/nos/sync/` - Sync metadata cache

Storage using existing `fsatomic` patterns:

```go
type SyncStore struct {
    devicesPath string
    statePath   string
    mu          sync.RWMutex
    devices     map[string]*DeviceToken
}

func (s *SyncStore) SaveDevice(d *DeviceToken) error {
    return fsatomic.WithLock(s.devicesPath, func() error {
        return fsatomic.SaveJSON(context.TODO(), s.devicesPath, s.devices, 0o600)
    })
}
```

### 8. OpenAPI Specification Updates

**File**: `backend/nosd/openapi.yaml`

Add sync endpoints and schemas:

```yaml
paths:
  /api/v1/sync/devices/register:
    post:
      summary: Register a sync device
      tags: [sync]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeviceRegisterRequest'
      responses:
        '201':
          description: Device registered
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DeviceRegisterResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '403':
          description: 2FA required
          
  # ... additional endpoints

components:
  schemas:
    DeviceRegisterRequest:
      type: object
      properties:
        device_name:
          type: string
          maxLength: 64
        device_type:
          type: string
          enum: [windows, linux, android, ios]
        os_version:
          type: string
        client_version:
          type: string
      required: [device_name, device_type]
      
    DeviceRegisterResponse:
      type: object
      properties:
        device_id:
          type: string
        device_token:
          type: string
        refresh_token:
          type: string
        expires_at:
          type: string
          format: date-time
```

### 9. Web UI Changes

#### 9.1 New Page: Sync Devices

**File**: `web/src/pages/SyncDevices.tsx`

```typescript
// Device management UI
// - List connected devices
// - Show last sync time, device type, IP
// - Revoke device access
// - QR code for mobile app setup
```

#### 9.2 Share Settings Extension

**File**: `web/src/components/shares/ShareForm.tsx`

Add sync configuration tab:

```typescript
// Add to share form:
// - Enable/disable sync toggle
// - Max sync size limit
// - Exclude patterns
```

#### 9.3 New Settings Section

**File**: `web/src/pages/SettingsSync.tsx`

```typescript
// Global sync settings
// - Default bandwidth limits
// - Sync schedule (if polling)
// - Conflict resolution preferences
```

---

## Client Applications

### Desktop Client (Windows/Linux)

#### Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Core Engine | **Go** | Cross-platform, same as backend, good for file I/O |
| GUI | **Wails** or **Fyne** | Native-feeling UI with Go backend |
| File System | Windows: **CloudAPI/Shell Extensions**, Linux: **FUSE** | Virtual folder integration |
| Sync | Custom protocol over HTTPS/WebDAV | Works through firewalls |

#### Architecture

```
┌───────────────────────────────────────────────────────────┐
│                    NithronSync Desktop                      │
├───────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐                 │
│  │    GUI Layer    │  │  System Tray    │                 │
│  │   (Wails/Fyne)  │  │    Icon         │                 │
│  └────────┬────────┘  └────────┬────────┘                 │
│           │                    │                           │
│           ▼                    ▼                           │
│  ┌─────────────────────────────────────────┐              │
│  │            Sync Controller               │              │
│  │  - Sync state machine                   │              │
│  │  - Conflict resolution                  │              │
│  │  - Queue management                     │              │
│  └─────────────────┬───────────────────────┘              │
│                    │                                       │
│  ┌─────────────────┴───────────────────────┐              │
│  │            File Watcher                  │              │
│  │  - fsnotify (cross-platform)            │              │
│  │  - Debounce rapid changes               │              │
│  └─────────────────┬───────────────────────┘              │
│                    │                                       │
│  ┌─────────────────┴───────────────────────┐              │
│  │         HTTP/WebDAV Client              │              │
│  │  - Device token auth                    │              │
│  │  - Chunked transfers                    │              │
│  │  - Delta sync                           │              │
│  └─────────────────┬───────────────────────┘              │
│                    │                                       │
│  ┌─────────────────┴───────────────────────┐              │
│  │     Virtual Filesystem (Optional)        │              │
│  │  Win: Cloud Filter API / Shell Ext      │              │
│  │  Linux: FUSE mount                      │              │
│  └─────────────────────────────────────────┘              │
└───────────────────────────────────────────────────────────┘
```

#### Key Features

1. **Sync Folder**: Traditional sync folder at `~/NithronSync/` or custom location
2. **Files On-Demand (Phase 2)**: 
   - Windows: Use Cloud Filter API for placeholders
   - Linux: FUSE filesystem with on-demand fetch
3. **Smart Sync**: Prioritize recently accessed files
4. **Conflict Handling**: User notification with options

#### Configuration Storage

- Windows: `%APPDATA%\NithronSync\config.json`
- Linux: `~/.config/nithronos/sync.json`

### Mobile App (Android/iOS)

#### Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Framework | **Flutter** or **React Native** | Cross-platform, single codebase |
| Storage | SQLite | Offline metadata cache |
| Networking | Platform HTTP client | Native performance |
| Background | WorkManager (Android), BGTaskScheduler (iOS) | Battery-efficient |

#### Key Features

1. **File Browser**: Navigate synced shares
2. **On-Demand Download**: Files fetched when opened
3. **Offline Favorites**: Mark files for offline access
4. **Camera Backup**: Auto-upload photos/videos
5. **Share Integration**: Open files from other apps

#### UI Screens

1. **Home**: Recent files, sync status
2. **Browse**: Folder hierarchy
3. **Offline**: Downloaded files
4. **Photos**: Camera backup settings
5. **Settings**: Account, sync preferences
6. **Devices**: View/manage connected devices

---

## Security Model

### Authentication Flow

```
Device Registration:
                                                          
┌─────────┐        ┌─────────────┐        ┌─────────────┐
│  User   │        │  Sync App   │        │  NithronOS  │
└────┬────┘        └──────┬──────┘        └──────┬──────┘
     │                    │                      │
     │  Enter server URL  │                      │
     │───────────────────▶│                      │
     │                    │                      │
     │                    │  POST /auth/login    │
     │                    │  (username/password) │
     │                    │─────────────────────▶│
     │                    │                      │
     │                    │  401 (2FA required)  │
     │                    │◀─────────────────────│
     │                    │                      │
     │  Enter TOTP code   │                      │
     │───────────────────▶│                      │
     │                    │                      │
     │                    │  POST /auth/login    │
     │                    │  (with TOTP)         │
     │                    │─────────────────────▶│
     │                    │                      │
     │                    │  200 (session token) │
     │                    │◀─────────────────────│
     │                    │                      │
     │                    │  POST /sync/devices/ │
     │                    │       register       │
     │                    │─────────────────────▶│
     │                    │                      │
     │                    │  201 (device token)  │
     │                    │◀─────────────────────│
     │                    │                      │
     │                    │  Use device token    │
     │                    │  for all future      │
     │                    │  sync requests       │
     │                    │                      │
```

### Token Security

| Token Type | Prefix | Lifetime | Storage |
|------------|--------|----------|---------|
| Device Token | `nos_dt_` | 90 days | Secure storage (Keychain/Credential Manager) |
| Refresh Token | `nos_rt_` | 1 year | Secure storage |
| Session Token | `nos_st_` | 15 min | Memory only |

### Data Security

1. **TLS Required**: All sync traffic over HTTPS
2. **Certificate Pinning**: Optional, configurable
3. **Local Encryption**: Mobile apps encrypt cached files with device key
4. **Token Revocation**: Immediate effect via JWT blacklist

### Threat Mitigations

| Threat | Mitigation |
|--------|------------|
| Stolen device token | Revoke from web UI; short expiry with refresh |
| Man-in-the-middle | TLS 1.3; optional cert pinning |
| Data at rest | OS-level encryption (BitLocker/LUKS) |
| Brute force | Rate limiting on device registration |
| Token replay | IP binding (optional); refresh rotation |

---

## Implementation Plan

### Phase 1: Server Foundation (4-6 weeks)

#### Sprint 1.1: Core API (2 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Create `pkg/sync` package structure | 1d | Backend |
| Implement device token model | 2d | Backend |
| Add device registration endpoint | 2d | Backend |
| Add device token auth middleware | 1d | Backend |
| Extend shares with sync settings | 1d | Backend |
| Add basic change tracking | 2d | Backend |
| Unit tests | 2d | Backend |

**Deliverables**: Device registration API, basic auth working

#### Sprint 1.2: File Operations (2 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Implement WebDAV handler | 3d | Backend |
| Add `/sync/changes` endpoint | 2d | Backend |
| Add file metadata endpoint | 1d | Backend |
| Integrate with Caddy config | 1d | DevOps |
| Add block hash endpoint for delta sync | 2d | Backend |
| Integration tests | 1d | Backend |

**Deliverables**: Full sync API, WebDAV working

#### Sprint 1.3: Web UI (1-2 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Devices management page | 2d | Frontend |
| Extend share form with sync options | 1d | Frontend |
| Add sync settings page | 1d | Frontend |
| QR code generation for mobile setup | 0.5d | Frontend |
| Documentation | 1d | All |

**Deliverables**: Complete web UI for sync management

### Phase 2: Desktop Client MVP (6-8 weeks)

#### Sprint 2.1: Core Engine (3 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Set up Go project structure | 1d | Client |
| Implement API client library | 3d | Client |
| Build sync state machine | 3d | Client |
| Implement file watcher (fsnotify) | 2d | Client |
| Add conflict detection | 2d | Client |
| Implement queue management | 2d | Client |
| Local state persistence (SQLite) | 2d | Client |

**Deliverables**: Headless sync engine

#### Sprint 2.2: Desktop Integration (3 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Wails/Fyne UI setup | 2d | Client |
| System tray implementation | 2d | Client |
| Settings/preferences UI | 2d | Client |
| Sync folder configuration | 1d | Client |
| Context menu integration (basic) | 2d | Client |
| Auto-start setup | 1d | Client |
| Installers (Windows MSI, Linux DEB/RPM) | 2d | DevOps |
| Testing & bug fixes | 3d | Client |

**Deliverables**: Installable desktop client for Windows/Linux

#### Sprint 2.3: Polish & Beta (2 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Performance optimization | 2d | Client |
| Error handling & logging | 1d | Client |
| Bandwidth throttling | 1d | Client |
| Auto-updater | 2d | Client |
| Beta testing | 4d | QA |

**Deliverables**: Beta-ready desktop client

### Phase 3: Mobile App MVP (8-10 weeks)

#### Sprint 3.1: App Foundation (3 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Flutter/RN project setup | 1d | Mobile |
| Authentication flow | 3d | Mobile |
| API client integration | 2d | Mobile |
| File browser UI | 3d | Mobile |
| Download/view files | 2d | Mobile |
| Offline storage layer | 2d | Mobile |

**Deliverables**: Basic file browser app

#### Sprint 3.2: Core Features (3 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| Upload files/photos | 3d | Mobile |
| Camera backup feature | 3d | Mobile |
| Favorites/offline files | 2d | Mobile |
| Settings screens | 2d | Mobile |
| Share sheet integration | 2d | Mobile |
| Push notifications | 1d | Mobile |

**Deliverables**: Feature-complete mobile app

#### Sprint 3.3: Polish & Release (2-4 weeks)

| Task | Effort | Owner |
|------|--------|-------|
| iOS-specific refinements | 3d | Mobile |
| Android-specific refinements | 3d | Mobile |
| Performance testing | 2d | QA |
| App Store preparation | 3d | Mobile |
| Beta testing | 5d | QA |

**Deliverables**: App Store-ready mobile apps

### Phase 4: Advanced Features (Ongoing)

| Feature | Effort | Phase |
|---------|--------|-------|
| Files On-Demand (Windows Cloud API) | 4 weeks | 4.1 |
| Files On-Demand (Linux FUSE) | 3 weeks | 4.1 |
| Delta sync implementation | 2 weeks | 4.2 |
| Real-time sync (WebSocket) | 2 weeks | 4.2 |
| Shared links generation | 1 week | 4.3 |
| Team/family sharing | 3 weeks | 4.3 |
| macOS client | 4 weeks | 4.4 |

---

## Future Enhancements

### Version 2.0+

1. **End-to-End Encryption**: Client-side encryption before upload
2. **LAN Sync**: Direct device-to-device sync on same network
3. **Shared Folders**: Collaborative folders with multiple users
4. **Version History**: Browse file versions from client apps
5. **Smart Sync**: ML-based prediction of file access patterns
6. **Backup Integration**: Integrate with NithronOS backup system
7. **macOS Client**: Native macOS support with Finder integration
8. **Web File Manager**: Browser-based file access

### Integration Opportunities

1. **Nextcloud Integration**: Import from Nextcloud
2. **Photo Management**: AI tagging, albums
3. **Document Preview**: In-app document viewing
4. **Search**: Full-text search across synced files

---

## Appendix A: Technology Comparison

### Desktop Framework Options

| Option | Pros | Cons | Recommendation |
|--------|------|------|----------------|
| **Wails (Go + Web)** | Native perf, Go backend | Newer ecosystem | ✅ Recommended |
| **Fyne (Go)** | Pure Go, simple | Less polished UI | Good fallback |
| **Electron** | Mature, rich ecosystem | Memory heavy | Avoid |
| **Tauri (Rust)** | Low memory, modern | Learning curve | Consider |

### Mobile Framework Options

| Option | Pros | Cons | Recommendation |
|--------|------|------|----------------|
| **Flutter** | Fast, beautiful UI | Dart learning | ✅ Recommended |
| **React Native** | JS ecosystem | Performance concerns | Alternative |
| **Native (Swift/Kotlin)** | Best performance | 2x development | Not for MVP |

### Sync Protocol Options

| Option | Pros | Cons | Recommendation |
|--------|------|------|----------------|
| **WebDAV** | Standard, wide support | Limited delta | ✅ For v1 |
| **Custom REST** | Flexible, efficient | More work | Hybrid approach |
| **rsync-over-SSH** | Proven, efficient | Complex firewall | Not for mobile |
| **Syncthing Protocol** | Decentralized, tested | Over-complex | Consider for P2P |

---

## Appendix B: Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| WebDAV performance issues | Medium | High | Custom binary protocol fallback |
| Files On-Demand complexity | High | Medium | Defer to Phase 4 |
| Mobile battery drain | Medium | High | Aggressive batching, WorkManager |
| Large file sync failures | Medium | Medium | Chunked uploads, resume |
| Token security breach | Low | High | Short expiry, rotation, monitoring |
| App Store rejection | Low | High | Early review, compliance check |

---

## Appendix C: Related Roadmap Items

This plan aligns with existing NithronOS roadmap items:

- **A6 — Remote Access Plus**: Device tokens, 2FA gating *(direct dependency)*
- **A10 — Desktop Companion**: LAN discovery, share mapping, notifications *(this is it)*
- **A3 — Backup & Replication v2**: S3/cloud sync patterns *(shared infrastructure)*

---

## Appendix D: File Structure Summary

### New Files to Create

```
backend/nosd/
├── pkg/sync/
│   ├── device_manager.go
│   ├── change_tracker.go
│   ├── delta.go
│   ├── webdav.go
│   ├── types.go
│   └── store.go
└── internal/server/
    ├── sync_handler.go
    └── webdav_handler.go

web/src/
├── pages/
│   ├── SyncDevices.tsx
│   └── SettingsSync.tsx
└── api/
    └── sync.ts

clients/
├── desktop/              # New repository or monorepo path
│   ├── cmd/
│   │   └── nithronos-sync/
│   ├── internal/
│   │   ├── api/
│   │   ├── sync/
│   │   ├── watcher/
│   │   └── ui/
│   ├── go.mod
│   └── Makefile
└── mobile/               # New repository or monorepo path
    ├── lib/
    │   ├── api/
    │   ├── models/
    │   ├── screens/
    │   └── services/
    ├── pubspec.yaml
    └── README.md
```

### Files to Modify

```
backend/nosd/
├── pkg/auth/token.go           # Add sync scopes
├── internal/shares/store.go     # Add sync fields
├── internal/server/router.go    # Add sync routes
└── openapi.yaml                 # Add sync endpoints

web/src/
├── components/shares/ShareForm.tsx  # Add sync options
├── config/nav.ts                    # Add sync nav items
└── App.tsx                          # Add sync routes

packaging/
├── deb/caddy-nithronos/Caddyfile   # Add WebDAV routes
└── deb/nosd/debian/conffiles        # Add sync config files
```

---

*Document Version: 1.0*  
*Last Updated: December 2024*  
*Authors: NithronOS Development Team*

