/**
 * nosd's sync API as it serves it (backend/nosd/pkg/sync/types.go,
 * docs/sync/api.md). Times are RFC 3339 strings.
 */

export type DeviceType =
  | 'windows'
  | 'linux'
  | 'macos'
  | 'android'
  | 'ios'
  | 'unknown';

/** GET /api/v1/sync/devices/{id}, an entry of GET /api/v1/sync/devices */
export interface Device {
  id: string;
  device_name: string;
  device_type: DeviceType;
  os_version?: string;
  client_version?: string;
  created_at: string;
  last_sync_at?: string;
  last_seen_at?: string;
  last_ip?: string;
  sync_count: number;
  bytes_synced: number;
  is_revoked: boolean;
}

export interface DeviceListResponse {
  devices: Device[] | null;
  count: number;
}

/** POST /api/v1/sync/devices/refresh: both tokens are replaced */
export interface RefreshResponse {
  device_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface SyncShare {
  share_id: string;
  share_name: string;
  share_path: string;
  sync_enabled: boolean;
  total_size: number;
  file_count: number;
}

export interface ShareListResponse {
  shares: SyncShare[] | null;
  count: number;
}

/** GET /api/v1/sync/config: the device's own record (device_id is this device) */
export interface SyncConfig {
  device_id: string;
  sync_shares: string[] | null;
  selective_paths: string[] | null;
  bandwidth_limit_kbps: number;
  pause_sync: boolean;
  sync_on_mobile_data: boolean;
}

/** GET /api/v1/sync/files/{share_id}/metadata */
export interface FileMetadata {
  path: string;
  size: number;
  mtime: string;
  hash: string;
  is_dir: boolean;
  mode?: number;
}

/** A WebDAV PROPFIND entry: what a folder listing shows */
export interface FolderEntry {
  /** Relative to the share, no leading slash; '' is the share itself */
  path: string;
  name: string;
  isDir: boolean;
  size: number;
  /** ISO time, or null when the server didn't say */
  modified: string | null;
  contentType: string | null;
}

export type ConflictResolution = 'keep_local' | 'keep_remote' | 'keep_both';

export interface FileVersion {
  hash: string;
  size: number;
  modified: string;
  modified_by?: string;
}

export interface SyncConflict {
  id: string;
  share_id: string;
  device_id: string;
  path: string;
  local_version: FileVersion;
  remote_version: FileVersion;
  detected_at: string;
  resolved: boolean;
  resolution?: ConflictResolution | 'merge';
  resolved_at?: string;
  resolved_by?: string;
}

export type ActivityAction =
  | 'upload'
  | 'download'
  | 'delete'
  | 'rename'
  | 'conflict';

export type ActivityStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

export interface SyncActivity {
  id: string;
  device_id: string;
  share_id: string;
  action: ActivityAction;
  path: string;
  old_path?: string;
  size?: number;
  status: ActivityStatus;
  error?: string;
  started_at: string;
  completed_at?: string;
  progress?: number;
}

export interface ActivityListResponse {
  activities: SyncActivity[] | null;
  total: number;
  page: number;
  page_size: number;
}

/** nosd's error body: {"error": {"code", "message"}} */
export interface ErrorBody {
  error?: {code?: string; message?: string};
}
