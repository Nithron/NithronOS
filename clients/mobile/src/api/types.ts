/**
 * NithronSync API Types
 * Shared type definitions for API communication
 */

// Device types
export type DeviceType = 'desktop' | 'mobile' | 'tablet' | 'other';
export type DevicePlatform = 'windows' | 'linux' | 'macos' | 'ios' | 'android' | 'other';

// Authentication
export interface DeviceRegistration {
  name: string;
  type: DeviceType;
  platform: DevicePlatform;
  model?: string;
  osVersion?: string;
}

export interface DeviceTokenResponse {
  device_id: string;
  device_token: string;
  refresh_token: string;
  expires_at: string;
  scopes: string[];
}

export interface RefreshTokenResponse {
  device_token: string;
  expires_at: string;
}

export interface Device {
  id: string;
  name: string;
  type: DeviceType;
  platform: DevicePlatform;
  created_at: string;
  last_seen: string;
  last_sync?: string;
  sync_bytes?: number;
  is_online: boolean;
}

// Sync shares
export interface SyncShare {
  share_id: string;
  name: string;
  path: string;
  sync_enabled: boolean;
  sync_max_size?: number;
  sync_exclude?: string[];
  total_size?: number;
  file_count?: number;
}

// Sync configuration
export interface SyncConfig {
  device_id: string;
  enabled_shares: string[];
  bandwidth_limit?: number;
  pause_sync: boolean;
  selective_sync: Record<string, string[]>;
  sync_on_wifi_only: boolean;
  sync_on_metered: boolean;
  auto_upload_photos: boolean;
  auto_upload_videos: boolean;
}

// File metadata
export interface FileMetadata {
  path: string;
  name: string;
  is_dir: boolean;
  size: number;
  modified: string;
  hash?: string;
  mime_type?: string;
  thumbnail_url?: string;
}

// File change tracking
export interface FileChange {
  path: string;
  change_type: 'created' | 'modified' | 'deleted' | 'renamed';
  old_path?: string;
  metadata?: FileMetadata;
  timestamp: string;
}

export interface ChangesResponse {
  changes: FileChange[];
  cursor: string;
  has_more: boolean;
}

// Sync state
export interface SyncState {
  device_id: string;
  share_id: string;
  cursor: string;
  last_sync: string;
  total_bytes: number;
  synced_bytes: number;
  pending_uploads: number;
  pending_downloads: number;
  conflicts: number;
}

// Conflict resolution
export interface SyncConflict {
  id: string;
  share_id: string;
  path: string;
  local_version: FileVersion;
  remote_version: FileVersion;
  detected_at: string;
  resolved: boolean;
  resolution?: 'keep_local' | 'keep_remote' | 'keep_both' | 'merge';
}

export interface FileVersion {
  hash: string;
  size: number;
  modified: string;
  modified_by?: string;
}

// Activity history
export interface SyncActivity {
  id: string;
  device_id: string;
  share_id: string;
  action: 'upload' | 'download' | 'delete' | 'rename' | 'conflict';
  path: string;
  old_path?: string;
  size?: number;
  status: 'pending' | 'in_progress' | 'completed' | 'failed';
  error?: string;
  started_at: string;
  completed_at?: string;
  progress?: number;
}

export interface ActivityListResponse {
  activities: SyncActivity[];
  total: number;
  page: number;
  page_size: number;
}

// Block hashing for delta sync
export interface BlockHash {
  offset: number;
  size: number;
  weak: number; // Adler-32
  strong: string; // SHA-256
}

export interface BlockHashRequest {
  path: string;
  block_size?: number;
}

export interface BlockHashResponse {
  path: string;
  size: number;
  block_size: number;
  blocks: BlockHash[];
}

// Transfer progress
export interface TransferProgress {
  path: string;
  direction: 'upload' | 'download';
  total_bytes: number;
  transferred_bytes: number;
  speed_bps: number;
  eta_seconds?: number;
}

// Error types
export interface APIError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

// Server discovery
export interface ServerInfo {
  url: string;
  name: string;
  version: string;
  features: string[];
}

