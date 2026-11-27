# NithronSync API Reference

The API sync clients use, as nosd serves it. The wire types are in
`backend/nosd/pkg/sync/types.go`; `clients/sync-core/api` is the client, and
its tests decode these shapes. The full route list is in
`docs/api/openapi.json`.

## Authentication

Every endpoint below `/api/v1/sync/` (except pairing and refresh) takes
either a **device token** or the web interface's signed-in session (cookie
plus `X-CSRF-Token` for changes):

```http
Authorization: Bearer nos_dt_...
```

| Token | Prefix | Lifetime | Use |
|-------|--------|----------|-----|
| Device token | `nos_dt_` | 90 days | Every request, and WebDAV |
| Refresh token | `nos_rt_` | 365 days | Getting a new device token |

nOS's own certificate is self-signed: clients pin the server's key
(`server_key_sha256`, SHA-256 of the certificate's public key, hex) instead
of checking it against certificate authorities.

### Pairing link

The web interface hands a new device everything as one link (also the
mobile QR code):

```
nos://sync?server=<URL>&token=<device token>&refresh=<refresh token>&key=<server key>&name=<device name>
```

`refresh`, `key` and `name` may be missing. Built by
`web/src/lib/pairing.ts`, parsed by `clients/sync-core/api/pairing.go`.

## Devices

### Register a device

Signed-in session only (the pairing dialog does this).

```http
POST /api/v1/sync/devices/register
{"device_name": "Laptop", "device_type": "linux", "os_version": "...", "client_version": "..."}
```

`device_type`: `windows`, `linux`, `macos`, `android`, `ios`.

```json
201
{
  "device_id": "…",
  "device_token": "nos_dt_…",
  "refresh_token": "nos_rt_…",
  "expires_at": "2027-02-25T10:00:00Z",
  "server_key_sha256": "…"
}
```

`server_key_sha256` is missing when nosd couldn't read the web interface's
certificate.

### Refresh the device token

No authentication; the refresh token is the credential.

```http
POST /api/v1/sync/devices/refresh
{"refresh_token": "nos_rt_…"}
```

```json
{"device_token": "nos_dt_…", "refresh_token": "nos_rt_…", "expires_at": "…"}
```

Both tokens are replaced. A client retries a request once after a refresh.

### List, rename, revoke

```http
GET    /api/v1/sync/devices                  → {"devices": [Device], "count": n}
GET    /api/v1/sync/devices/{device_id}      → Device
PATCH  /api/v1/sync/devices/{device_id}      {"device_name": "…"} → Device
DELETE /api/v1/sync/devices/{device_id}
```

`Device`: `id`, `device_name`, `device_type`, `os_version`,
`client_version`, `created_at`, `last_sync_at`, `last_seen_at`, `last_ip`,
`sync_count`, `bytes_synced`, `is_revoked`.

## Shares

```http
GET /api/v1/sync/shares
```

```json
{
  "shares": [
    {"share_id": "…", "share_name": "Documents", "share_path": "/srv/shares/Documents",
     "sync_enabled": true, "total_size": 123, "file_count": 4}
  ],
  "count": 1
}
```

The shares the device's user may read, except those with sync turned off.
A share with sync turned off answers 404 everywhere below.

## Changes

```http
GET /api/v1/sync/changes?share_id=<id>&cursor=<cursor>&limit=<1-1000>
```

```json
{
  "changes": [
    {"path": "docs/a.txt", "type": "modify", "size": 12, "mtime": "…", "hash": "<sha256>", "is_dir": false},
    {"path": "old.txt", "type": "delete", "mtime": "…"}
  ],
  "cursor": "<id>",
  "has_more": false
}
```

- `type`: `create`, `modify`, `delete` (and `rename` with `old_path`).
- No cursor: every file is listed as `create`.
- The cursor is a short id; nosd keeps the snapshot it stands for, per device
  and share (only the latest). An unknown cursor lists everything again, so
  clients decide by comparing `hash` with what they last synced.
- `has_more`: fetch again with the new cursor.

## Files

### Metadata

```http
GET  /api/v1/sync/files/{share_id}/metadata?path=<path>   → FileMetadata
POST /api/v1/sync/files/{share_id}/metadata  {"paths": ["a", "b"]} → {"files": [FileMetadata]}
```

`FileMetadata`: `path`, `size`, `mtime`, `hash`, `is_dir`, `mode`.

### Block hashes (delta transfers)

```http
POST /api/v1/sync/files/{share_id}/hash
{"path": "big.bin"}
```

```json
{"path": "big.bin", "size": 9437184, "hash": "<sha256>", "block_size": 4194304,
 "blocks": [{"offset": 0, "size": 4194304, "hash": "<sha256>", "weak": 123456789}]}
```

`weak` is Adler-32. nosd picks the block size; clients use the one it
returns.

### Delta upload

Owners of the share only.

```http
POST /api/v1/sync/files/{share_id}/delta
{"path": "big.bin", "sha256": "<sha256 of the new file>",
 "ops": [{"offset": 0, "size": 4194304}, {"data": "<base64>"}, {"offset": 8388608, "size": 1048576}]}
```

Each op copies `size` bytes at `offset` of the server's current file, or
appends `data`. nosd rebuilds the file next to the old one and replaces it
only if the result hashes to `sha256`.

| Status | Code | Meaning |
|--------|------|---------|
| 200 | | `{"path", "size", "sha256"}` |
| 404 | `sync.no_base` | No current file: upload it whole |
| 409 | `sync.delta_mismatch` | The server's file changed: upload it whole |
| 400 | `sync.delta_invalid` | An op outside the current file |

At most 64 MiB per request.

## Sync state

```http
GET /api/v1/sync/state/{share_id}
PUT /api/v1/sync/state/{share_id}   {"cursor": "…", "total_files": n, "total_bytes": n}
```

A device's own record (`device_id`, `share_id`, `cursor`, `last_sync`,
`total_files`, `total_bytes`); the PUT also counts toward the device's sync
statistics.

## Device settings, conflicts, activity

```http
GET /api/v1/sync/config                    → SyncConfig of the calling device
PUT /api/v1/sync/config  SyncConfig        → SyncConfig
```

`SyncConfig`: `device_id` (the calling device: how a client learns its own
id, which the pairing link doesn't carry), `sync_shares`, `selective_paths`,
`bandwidth_limit_kbps`, `pause_sync`, `sync_on_mobile_data`.

```http
GET /api/v1/sync/conflicts?share_id=<id>&unresolved_only=true  → [Conflict]
GET /api/v1/sync/conflicts/{conflict_id}                       → Conflict
PUT /api/v1/sync/conflicts/{conflict_id}  {"resolution": "keep_local" | "keep_remote" | "keep_both" | "merge"}
```

`Conflict`: `id`, `share_id`, `device_id`, `path`, `local_version`,
`remote_version`, `detected_at`, `resolved`, `resolution`, `resolved_at`,
`resolved_by`.

```http
GET /api/v1/sync/activity?share_id=<id>&page=1&page_size=50
    → {"activities": [Activity], "total": n, "page": 1, "page_size": 50}
GET /api/v1/sync/activity/recent?limit=20   → [Activity]
GET /api/v1/sync/activity/stats
```

The calling device's records (a signed-in session sees its user's devices).

## WebDAV

```
/dav/{share_id}/{path}
```

Methods: `GET`, `HEAD`, `PUT`, `DELETE`, `PROPFIND`, `PROPPATCH`, `MKCOL`,
`COPY`, `MOVE`, `LOCK`, `UNLOCK`, `OPTIONS`. Readers may read; owners may
also change.

Authentication: the device token as `Authorization: Bearer nos_dt_…`, or as
the password of Basic authentication with any user name (file managers). A
request without one gets `401` with a Basic challenge.

```bash
curl -u "any:nos_dt_…" -X PROPFIND -H "Depth: 1" https://nas.local/dav/<share_id>/
curl -u "any:nos_dt_…" -T report.pdf https://nas.local/dav/<share_id>/docs/report.pdf
```

A `PUT` into a folder that doesn't exist answers `409`; create it with
`MKCOL` first.

## Errors

```json
{"error": {"code": "share.not_found", "message": "Share not found"}}
```

| Code | Status | Meaning |
|------|--------|---------|
| `auth.required` | 401 | No or invalid token or session |
| `input.invalid` | 400 | A missing or malformed field, or a path outside the share |
| `share.not_found` | 404 | No such share, not readable, or its sync is off |
| `device.limit` | 409 | The user has as many devices as allowed |
| `sync.changes_failed` | 500 | The share couldn't be scanned |
| `sync.delta_mismatch` | 409 | See delta upload |

Paths are always relative to the share; `..` can't leave it, and symbolic
links that point outside it are refused.
