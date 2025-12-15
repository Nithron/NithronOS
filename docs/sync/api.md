# NithronSync API Reference

This document describes the REST API endpoints for NithronSync file synchronization.

## Authentication

### Device Token Authentication

Most sync endpoints require device token authentication. After registering a device, you receive an access token that must be included in the `Authorization` header:

```http
Authorization: Bearer nos_at_...
```

### Token Types

| Token Type | Prefix | Lifetime | Purpose |
|------------|--------|----------|---------|
| Access Token | `nos_at_` | 24 hours | API requests |
| Refresh Token | `nos_rt_` | 90 days | Obtain new access tokens |
| Device Token | `nos_dt_` | Indefinite | Stored on device, paired with refresh token |

## Device Management

### Register Device

Register a new sync device and receive authentication tokens.

**Endpoint:** `POST /api/v1/sync/devices/register`

**Authentication:** User session (cookie auth)

**Request:**
```json
{
  "device_name": "My Laptop",
  "device_type": "windows",
  "client_version": "1.0.0",
  "os_version": "Windows 11 23H2"
}
```

**Response:**
```json
{
  "device_token": {
    "id": "dt_abc123",
    "user_id": "usr_xyz",
    "device_name": "My Laptop",
    "device_type": "windows",
    "scopes": ["sync.read", "sync.write"],
    "created_at": "2024-01-15T10:30:00Z"
  },
  "access_token": "nos_at_...",
  "refresh_token": "nos_rt_...",
  "access_expires_at": "2024-01-16T10:30:00Z",
  "refresh_expires_at": "2024-04-15T10:30:00Z"
}
```

**Device Types:**
- `windows` — Windows desktop
- `linux` — Linux desktop
- `macos` — macOS desktop
- `ios` — iPhone/iPad
- `android` — Android phone/tablet

### Refresh Access Token

Exchange a refresh token for a new access token.

**Endpoint:** `POST /api/v1/sync/devices/refresh`

**Request:**
```json
{
  "refresh_token": "nos_rt_...",
  "device_id": "dt_abc123"
}
```

**Response:**
```json
{
  "access_token": "nos_at_...",
  "refresh_token": "nos_rt_...",
  "access_expires_at": "2024-01-16T10:30:00Z",
  "refresh_expires_at": "2024-04-16T10:30:00Z"
}
```

> Note: The refresh token is rotated on each use for security.

### List Devices

Get all registered devices for the authenticated user.

**Endpoint:** `GET /api/v1/sync/devices`

**Authentication:** Device token (Bearer)

**Response:**
```json
{
  "devices": [
    {
      "id": "dt_abc123",
      "device_name": "My Laptop",
      "device_type": "windows",
      "client_version": "1.0.0",
      "last_seen_at": "2024-01-15T12:00:00Z",
      "last_sync_at": "2024-01-15T11:45:00Z",
      "status": "active",
      "ip_address": "192.168.1.100",
      "created_at": "2024-01-10T08:00:00Z"
    }
  ],
  "count": 1
}
```

### Get Device

Get details for a specific device.

**Endpoint:** `GET /api/v1/sync/devices/{device_id}`

**Response:** Same as single device in list response.

### Update Device

Update device name or configuration.

**Endpoint:** `PATCH /api/v1/sync/devices/{device_id}`

**Request:**
```json
{
  "device_name": "Work Laptop"
}
```

### Revoke Device

Remove a device and invalidate its tokens.

**Endpoint:** `DELETE /api/v1/sync/devices/{device_id}`

**Response:** `204 No Content`

## Sync Configuration

### List Sync-Enabled Shares

Get all shares that have sync enabled.

**Endpoint:** `GET /api/v1/sync/shares`

**Response:**
```json
[
  {
    "id": "share_abc123",
    "name": "Documents",
    "path": "/mnt/pool1/documents",
    "sync_enabled": true,
    "sync_max_size": 10737418240,
    "sync_exclude": ["*.tmp", "~$*", ".git/**"]
  }
]
```

### Get Sync Configuration

Get per-device sync configuration.

**Endpoint:** `GET /api/v1/sync/config`

**Response:**
```json
{
  "device_id": "dt_abc123",
  "enabled": true,
  "sync_shares": ["share_abc123", "share_def456"],
  "bandwidth_limit_kbps": 0,
  "sync_on_metered": false,
  "selective_sync": {
    "share_abc123": {
      "include_paths": ["/Work", "/Projects"],
      "exclude_paths": ["/Archive"]
    }
  }
}
```

### Update Sync Configuration

Update device sync settings.

**Endpoint:** `PUT /api/v1/sync/config`

**Request:**
```json
{
  "enabled": true,
  "sync_shares": ["share_abc123"],
  "bandwidth_limit_kbps": 5000,
  "sync_on_metered": false
}
```

## File Synchronization

### Get Changes

Fetch file changes since a cursor position.

**Endpoint:** `GET /api/v1/sync/changes`

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `share_id` | string | Required. Share to get changes for |
| `cursor` | string | Optional. Cursor from previous response |
| `limit` | int | Optional. Max changes to return (default: 1000) |

**Response:**
```json
{
  "changes": [
    {
      "path": "/Documents/report.docx",
      "type": "file",
      "action": "modified",
      "size": 45678,
      "mod_time": "2024-01-15T14:30:00Z",
      "content_hash": "sha256:abc123...",
      "version": 3
    },
    {
      "path": "/Images/photo.jpg",
      "type": "file",
      "action": "created",
      "size": 2345678,
      "mod_time": "2024-01-15T14:35:00Z",
      "content_hash": "sha256:def456..."
    },
    {
      "path": "/Old Folder",
      "type": "dir",
      "action": "deleted"
    }
  ],
  "cursor": "eyJ0cyI6MTcwNTMyNDIwMH0",
  "has_more": false
}
```

**Actions:**
- `created` — New file or folder
- `modified` — File content changed
- `deleted` — File or folder removed
- `moved` — File or folder renamed/moved (includes `previous_path`)

### Get File Metadata

Get metadata for a specific file or folder.

**Endpoint:** `GET /api/v1/sync/files/{share_id}/metadata`

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | string | Required. Path within share |
| `include_children` | bool | Include children for directories |

**Response (file):**
```json
{
  "path": "/Documents/report.docx",
  "type": "file",
  "size": 45678,
  "mod_time": "2024-01-15T14:30:00Z",
  "content_hash": "sha256:abc123...",
  "version": 3
}
```

**Response (directory with children):**
```json
{
  "path": "/Documents",
  "type": "dir",
  "mod_time": "2024-01-15T14:30:00Z",
  "children": [
    {"path": "/Documents/report.docx", "type": "file", "size": 45678},
    {"path": "/Documents/Invoices", "type": "dir"}
  ]
}
```

### Get Block Hashes

Get block-level hashes for delta sync.

**Endpoint:** `POST /api/v1/sync/files/{share_id}/hash`

**Request:**
```json
{
  "path": "/Documents/large-file.zip",
  "block_size": 4194304
}
```

**Response:**
```json
{
  "path": "/Documents/large-file.zip",
  "size": 104857600,
  "block_size": 4194304,
  "blocks": [
    {
      "index": 0,
      "offset": 0,
      "size": 4194304,
      "strong_hash": "sha256:abc123...",
      "weak_hash": 12345678
    },
    {
      "index": 1,
      "offset": 4194304,
      "size": 4194304,
      "strong_hash": "sha256:def456...",
      "weak_hash": 87654321
    }
  ]
}
```

### Get Sync State

Get current sync state for a share.

**Endpoint:** `GET /api/v1/sync/state/{share_id}`

**Response:**
```json
{
  "share_id": "share_abc123",
  "device_id": "dt_abc123",
  "last_sync_at": "2024-01-15T14:30:00Z",
  "cursor": "eyJ0cyI6MTcwNTMyNDIwMH0",
  "sync_status": "synced",
  "pending_uploads": 0,
  "pending_downloads": 2,
  "errors": []
}
```

### Update Sync State

Save sync state after completing operations.

**Endpoint:** `PUT /api/v1/sync/state/{share_id}`

**Request:**
```json
{
  "cursor": "eyJ0cyI6MTcwNTMyNDIwMH0",
  "sync_status": "synced"
}
```

## WebDAV Access

NithronSync provides WebDAV access for broader client compatibility.

**Base URL:** `/dav/{share_id}/`

### Authentication

Include device access token in the `Authorization` header:

```http
Authorization: Bearer nos_at_...
```

### Supported Methods

| Method | Description |
|--------|-------------|
| GET | Download file |
| PUT | Upload file |
| DELETE | Delete file/folder |
| MKCOL | Create folder |
| COPY | Copy file/folder |
| MOVE | Move/rename file/folder |
| PROPFIND | List directory or get file properties |
| PROPPATCH | Update properties |

### Example: List Directory

```http
PROPFIND /dav/share_abc123/Documents/ HTTP/1.1
Authorization: Bearer nos_at_...
Depth: 1
Content-Type: application/xml

<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:getcontentlength/>
    <D:getlastmodified/>
    <D:resourcetype/>
  </D:prop>
</D:propfind>
```

### Example: Upload File

```http
PUT /dav/share_abc123/Documents/newfile.txt HTTP/1.1
Authorization: Bearer nos_at_...
Content-Type: application/octet-stream
Content-Length: 1234

[file content]
```

## Error Responses

All error responses follow this format:

```json
{
  "error": "Human-readable error message",
  "code": "error_code",
  "details": {}
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | Invalid or missing authentication |
| `forbidden` | 403 | Insufficient permissions |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Conflict (e.g., file exists) |
| `rate_limited` | 429 | Too many requests |
| `internal_error` | 500 | Server error |
| `device_limit_reached` | 400 | Max devices per user exceeded |
| `token_expired` | 401 | Access token expired |
| `token_revoked` | 401 | Device token was revoked |
| `share_not_enabled` | 403 | Share doesn't have sync enabled |
| `file_too_large` | 413 | File exceeds max sync size |

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| Device registration | 10/hour per user |
| Token refresh | 60/hour per device |
| File changes | 120/minute per device |
| File operations | 1000/hour per device |

Rate limit headers are included in responses:
```http
X-RateLimit-Limit: 120
X-RateLimit-Remaining: 118
X-RateLimit-Reset: 1705325400
```

## Pagination

List endpoints support cursor-based pagination:

```json
{
  "data": [...],
  "cursor": "eyJ0cyI6MTcwNTMyNDIwMH0",
  "has_more": true
}
```

To get the next page, include `cursor` in your request query parameters.

