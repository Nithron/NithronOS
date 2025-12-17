/**
 * NithronSync API Client
 * Handles sync device management and file synchronization operations
 */

import http from '@/lib/nos-client';

// ============================================================================
// Types
// ============================================================================

export type DeviceType = 'windows' | 'linux' | 'macos' | 'android' | 'ios';

export interface SyncDevice {
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

export interface DeviceRegisterRequest {
  device_name: string;
  device_type: DeviceType;
  os_version?: string;
  client_version?: string;
}

export interface DeviceRegisterResponse {
  device_id: string;
  device_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface DeviceRefreshRequest {
  refresh_token: string;
}

export interface DeviceRefreshResponse {
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
  max_sync_size?: number;
  exclude_patterns?: string[];
}

export interface SyncConfig {
  device_id: string;
  sync_shares: string[];
  selective_paths?: string[];
  bandwidth_limit_kbps?: number;
  pause_sync: boolean;
  sync_on_mobile_data?: boolean;
}

export type FileChangeType = 'create' | 'modify' | 'delete' | 'rename';

export interface FileChange {
  path: string;
  type: FileChangeType;
  size?: number;
  mtime?: string;
  hash?: string;
  old_path?: string;
  is_dir?: boolean;
}

export interface ChangesResponse {
  changes: FileChange[];
  cursor: string;
  has_more: boolean;
}

export interface FileMetadata {
  path: string;
  size?: number;
  mtime?: string;
  hash?: string;
  is_dir?: boolean;
  mode?: number;
}

export interface BlockHash {
  offset: number;
  size: number;
  hash: string;
  weak?: number;
}

export interface BlockHashResponse {
  path: string;
  size: number;
  hash: string;
  block_size: number;
  blocks: BlockHash[];
}

export interface SyncState {
  device_id: string;
  share_id: string;
  cursor: string;
  last_sync: string;
  total_files: number;
  total_bytes: number;
}

export interface SyncStats {
  total_devices: number;
  token_cache_size: number;
  max_devices_per_user: number;
  device_token_ttl_sec: number;
}

// Conflict types
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
  resolution?: ConflictResolution;
  resolved_at?: string;
  resolved_by?: string;
}

// Activity types
export type ActivityAction = 'upload' | 'download' | 'delete' | 'rename' | 'conflict';
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
  activities: SyncActivity[];
  total: number;
  page: number;
  page_size: number;
}

export interface ActivityStats {
  total: number;
  uploads: number;
  downloads: number;
  deletes: number;
  conflicts: number;
  completed: number;
  failed: number;
  bytes_synced: number;
}

// Collaboration types
export type SharePermission = 'read' | 'write' | 'admin';
export type InviteStatus = 'pending' | 'accepted' | 'declined' | 'expired';

export interface FolderMember {
  user_id: string;
  username: string;
  permission: SharePermission;
  added_at: string;
  added_by: string;
}

export interface SharedFolder {
  id: string;
  share_id: string;
  path: string;
  name: string;
  owner_id: string;
  owner_name: string;
  created_at: string;
  members: FolderMember[];
}

export interface ShareInvite {
  id: string;
  shared_folder_id: string;
  folder_name: string;
  inviter_id: string;
  inviter_name: string;
  invitee_id: string;
  invitee_email?: string;
  permission: SharePermission;
  status: InviteStatus;
  message?: string;
  created_at: string;
  expires_at: string;
  responded_at?: string;
}

// ============================================================================
// API Functions
// ============================================================================

const BASE_PATH = '/v1/sync';

/**
 * Register a new sync device
 * Note: This requires user session authentication, not device token
 */
export async function registerDevice(request: DeviceRegisterRequest): Promise<DeviceRegisterResponse> {
  return http.post<DeviceRegisterResponse>(`${BASE_PATH}/devices/register`, request);
}

/**
 * Refresh device tokens using a refresh token
 */
export async function refreshDeviceToken(request: DeviceRefreshRequest): Promise<DeviceRefreshResponse> {
  return http.post<DeviceRefreshResponse>(`${BASE_PATH}/devices/refresh`, request);
}

/**
 * List all sync devices for the current user
 */
export async function listDevices(): Promise<{ devices: SyncDevice[]; count: number }> {
  return http.get<{ devices: SyncDevice[]; count: number }>(`${BASE_PATH}/devices`);
}

/**
 * Get details for a specific device
 */
export async function getDevice(deviceId: string): Promise<SyncDevice> {
  return http.get<SyncDevice>(`${BASE_PATH}/devices/${deviceId}`);
}

/**
 * Revoke a device's access
 */
export async function revokeDevice(deviceId: string): Promise<void> {
  return http.del<void>(`${BASE_PATH}/devices/${deviceId}`);
}

/**
 * Update a device (e.g., rename)
 */
export async function updateDevice(deviceId: string, updates: { device_name?: string }): Promise<SyncDevice> {
  return http.patch<SyncDevice>(`${BASE_PATH}/devices/${deviceId}`, updates);
}

/**
 * List sync-enabled shares accessible to the current user/device
 */
export async function listSyncShares(): Promise<{ shares: SyncShare[]; count: number }> {
  return http.get<{ shares: SyncShare[]; count: number }>(`${BASE_PATH}/shares`);
}

/**
 * Get sync configuration for the current device
 */
export async function getSyncConfig(): Promise<SyncConfig> {
  return http.get<SyncConfig>(`${BASE_PATH}/config`);
}

/**
 * Update sync configuration
 */
export async function updateSyncConfig(config: Partial<SyncConfig>): Promise<SyncConfig> {
  return http.put<SyncConfig>(`${BASE_PATH}/config`, config);
}

/**
 * Get file changes since the given cursor
 */
export async function getChanges(
  shareId: string,
  cursor?: string,
  limit?: number
): Promise<ChangesResponse> {
  const params: Record<string, string> = { share_id: shareId };
  if (cursor) params.cursor = cursor;
  if (limit) params.limit = limit.toString();
  
  return http.get<ChangesResponse>(`${BASE_PATH}/changes`, params);
}

/**
 * Get metadata for a single file
 */
export async function getFileMetadata(shareId: string, path: string): Promise<FileMetadata> {
  return http.get<FileMetadata>(`${BASE_PATH}/files/${shareId}/metadata`, { path });
}

/**
 * Get metadata for multiple files
 */
export async function getFilesMetadata(shareId: string, paths: string[]): Promise<{ files: FileMetadata[] }> {
  return http.post<{ files: FileMetadata[] }>(`${BASE_PATH}/files/${shareId}/metadata`, { paths });
}

/**
 * Get block hashes for delta sync
 */
export async function getBlockHashes(
  shareId: string,
  path: string,
  blockSize?: number
): Promise<BlockHashResponse> {
  return http.post<BlockHashResponse>(`${BASE_PATH}/files/${shareId}/hash`, {
    path,
    block_size: blockSize,
  });
}

/**
 * Get sync state for a share
 */
export async function getSyncState(shareId: string): Promise<SyncState> {
  return http.get<SyncState>(`${BASE_PATH}/state/${shareId}`);
}

/**
 * Update sync state for a share
 */
export async function updateSyncState(shareId: string, state: Partial<SyncState>): Promise<SyncState> {
  return http.put<SyncState>(`${BASE_PATH}/state/${shareId}`, state);
}

// ============================================================================
// Conflict API Functions
// ============================================================================

/**
 * List sync conflicts
 */
export async function listConflicts(shareId?: string, unresolvedOnly = true): Promise<SyncConflict[]> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  if (unresolvedOnly) params.unresolved_only = 'true';
  return http.get<SyncConflict[]>(`${BASE_PATH}/conflicts`, params);
}

/**
 * Get a specific conflict
 */
export async function getConflict(conflictId: string): Promise<SyncConflict> {
  return http.get<SyncConflict>(`${BASE_PATH}/conflicts/${conflictId}`);
}

/**
 * Resolve a conflict
 */
export async function resolveConflict(
  conflictId: string,
  resolution: ConflictResolution
): Promise<SyncConflict> {
  return http.put<SyncConflict>(`${BASE_PATH}/conflicts/${conflictId}`, { resolution });
}

// ============================================================================
// Activity API Functions
// ============================================================================

/**
 * List sync activity with pagination
 */
export async function listActivity(
  shareId?: string,
  page = 1,
  pageSize = 50
): Promise<ActivityListResponse> {
  const params: Record<string, string> = {
    page: page.toString(),
    page_size: pageSize.toString(),
  };
  if (shareId) params.share_id = shareId;
  return http.get<ActivityListResponse>(`${BASE_PATH}/activity`, params);
}

/**
 * Get recent sync activity
 */
export async function getRecentActivity(limit = 20): Promise<SyncActivity[]> {
  return http.get<SyncActivity[]>(`${BASE_PATH}/activity/recent`, { limit: limit.toString() });
}

/**
 * Get activity statistics
 */
export async function getActivityStats(shareId?: string): Promise<ActivityStats> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  return http.get<ActivityStats>(`${BASE_PATH}/activity/stats`, params);
}

// ============================================================================
// Collaboration API Functions
// ============================================================================

/**
 * List shared folders accessible to the user
 */
export async function listSharedFolders(): Promise<{ folders: SharedFolder[]; count: number }> {
  return http.get<{ folders: SharedFolder[]; count: number }>(`${BASE_PATH}/shared-folders`);
}

/**
 * Create a new shared folder
 */
export async function createSharedFolder(
  shareId: string,
  path: string,
  name: string,
  ownerName: string
): Promise<SharedFolder> {
  return http.post<SharedFolder>(`${BASE_PATH}/shared-folders`, {
    share_id: shareId,
    path,
    name,
    owner_name: ownerName,
  });
}

/**
 * Get a specific shared folder
 */
export async function getSharedFolder(folderId: string): Promise<SharedFolder> {
  return http.get<SharedFolder>(`${BASE_PATH}/shared-folders/${folderId}`);
}

/**
 * Delete a shared folder
 */
export async function deleteSharedFolder(folderId: string): Promise<void> {
  return http.del<void>(`${BASE_PATH}/shared-folders/${folderId}`);
}

/**
 * Add a member to a shared folder
 */
export async function addFolderMember(
  folderId: string,
  userId: string,
  username: string,
  permission: SharePermission
): Promise<SharedFolder> {
  return http.post<SharedFolder>(`${BASE_PATH}/shared-folders/${folderId}/members`, {
    user_id: userId,
    username,
    permission,
  });
}

/**
 * Remove a member from a shared folder
 */
export async function removeFolderMember(folderId: string, userId: string): Promise<void> {
  return http.del<void>(`${BASE_PATH}/shared-folders/${folderId}/members/${userId}`);
}

/**
 * Update a member's permission
 */
export async function updateFolderMember(
  folderId: string,
  userId: string,
  permission: SharePermission
): Promise<SharedFolder> {
  return http.put<SharedFolder>(`${BASE_PATH}/shared-folders/${folderId}/members/${userId}`, {
    permission,
  });
}

/**
 * List pending invitations
 */
export async function listPendingInvites(): Promise<{ invites: ShareInvite[]; count: number }> {
  return http.get<{ invites: ShareInvite[]; count: number }>(`${BASE_PATH}/invites`);
}

/**
 * Create an invitation
 */
export async function createInvite(
  folderId: string,
  inviterName: string,
  inviteeId: string,
  inviteeEmail: string,
  permission: SharePermission,
  message?: string
): Promise<ShareInvite> {
  return http.post<ShareInvite>(`${BASE_PATH}/invites`, {
    folder_id: folderId,
    inviter_name: inviterName,
    invitee_id: inviteeId,
    invitee_email: inviteeEmail,
    permission,
    message,
  });
}

/**
 * Accept an invitation
 */
export async function acceptInvite(inviteId: string, username: string): Promise<ShareInvite> {
  return http.put<ShareInvite>(`${BASE_PATH}/invites/${inviteId}/accept`, { username });
}

/**
 * Decline an invitation
 */
export async function declineInvite(inviteId: string): Promise<ShareInvite> {
  return http.put<ShareInvite>(`${BASE_PATH}/invites/${inviteId}/decline`, {});
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Format device type for display
 */
export function formatDeviceType(type: DeviceType): string {
  const labels: Record<DeviceType, string> = {
    windows: 'Windows',
    linux: 'Linux',
    macos: 'macOS',
    android: 'Android',
    ios: 'iOS',
  };
  return labels[type] || type;
}

/**
 * Get icon name for device type
 */
export function getDeviceIcon(type: DeviceType): string {
  const icons: Record<DeviceType, string> = {
    windows: 'monitor',
    linux: 'terminal',
    macos: 'laptop',
    android: 'smartphone',
    ios: 'smartphone',
  };
  return icons[type] || 'device-unknown';
}

/**
 * Format bytes for display
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * Format relative time
 */
export function formatRelativeTime(dateString: string | undefined): string {
  if (!dateString) return 'Never';
  
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);
  
  if (diffSec < 60) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  
  return date.toLocaleDateString();
}

/**
 * Check if a device is considered active (seen in last 24 hours)
 */
export function isDeviceActive(device: SyncDevice): boolean {
  if (!device.last_seen_at) return false;
  const lastSeen = new Date(device.last_seen_at);
  const now = new Date();
  const diffMs = now.getTime() - lastSeen.getTime();
  const diffHours = diffMs / (1000 * 60 * 60);
  return diffHours < 24;
}

/**
 * Generate QR code data for mobile device registration
 */
export function generateDeviceQRData(serverUrl: string, setupToken: string): string {
  return JSON.stringify({
    type: 'nithronos-sync',
    version: 1,
    server: serverUrl,
    token: setupToken,
  });
}

// ============================================================================
// React Query Keys (for cache management)
// ============================================================================

export const syncKeys = {
  all: ['sync'] as const,
  devices: () => [...syncKeys.all, 'devices'] as const,
  device: (id: string) => [...syncKeys.devices(), id] as const,
  shares: () => [...syncKeys.all, 'shares'] as const,
  config: () => [...syncKeys.all, 'config'] as const,
  changes: (shareId: string) => [...syncKeys.all, 'changes', shareId] as const,
  state: (shareId: string) => [...syncKeys.all, 'state', shareId] as const,
  conflicts: () => [...syncKeys.all, 'conflicts'] as const,
  conflict: (id: string) => [...syncKeys.conflicts(), id] as const,
  activity: () => [...syncKeys.all, 'activity'] as const,
  activityRecent: () => [...syncKeys.activity(), 'recent'] as const,
  activityStats: () => [...syncKeys.activity(), 'stats'] as const,
  sharedFolders: () => [...syncKeys.all, 'shared-folders'] as const,
  sharedFolder: (id: string) => [...syncKeys.sharedFolders(), id] as const,
  invites: () => [...syncKeys.all, 'invites'] as const,
};

