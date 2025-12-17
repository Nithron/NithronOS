/**
 * Offline sync API client for NithronSync
 */

import http from '@/lib/nos-client';

const BASE_PATH = '/api/v1/sync/offline';

// Types
export type OperationType = 'create' | 'modify' | 'delete' | 'rename' | 'move';
export type OperationStatus = 'pending' | 'in_progress' | 'completed' | 'failed' | 'conflict' | 'cancelled';
export type SyncMode = 'online' | 'offline' | 'syncing';
export type NetworkStatus = 'connected' | 'disconnected' | 'metered';

export interface QueuedOperation {
  id: string;
  share_id: string;
  path: string;
  old_path?: string;
  type: OperationType;
  status: OperationStatus;
  priority: number;
  size: number;
  hash?: string;
  local_version: number;
  remote_version?: number;
  created_at: string;
  modified_at: string;
  attempt_count: number;
  last_attempt?: string;
  error?: string;
}

export interface QueueStats {
  total: number;
  pending: number;
  pending_bytes: number;
  in_progress: number;
  completed: number;
  failed: number;
  conflicts: number;
}

export interface OfflineStatus {
  mode: SyncMode;
  network_status: NetworkStatus;
  last_sync: string;
  queue_stats: QueueStats;
  conflict_count: number;
}

export interface LocalFileState {
  share_id: string;
  path: string;
  local_hash: string;
  remote_hash?: string;
  local_version: number;
  remote_version: number;
  size: number;
  modified_at: string;
  synced_at?: string;
  is_available: boolean;
  is_pinned: boolean;
  is_placeholder: boolean;
}

export interface SyncConflict {
  id: string;
  share_id: string;
  path: string;
  local_version: FileVersion;
  remote_version: FileVersion;
  base_version?: FileVersion;
  conflict_type: 'modify_modify' | 'modify_delete' | 'delete_modify' | 'create_create' | 'move_move';
  resolution?: 'keep_local' | 'keep_remote' | 'keep_both' | 'merge' | 'manual';
  resolved_at?: string;
  resolved_by?: string;
  created_at: string;
}

export interface FileVersion {
  version: number;
  hash: string;
  size: number;
  modified_at: string;
  modified_by?: string;
  device_id?: string;
}

// API Functions

/**
 * Get offline sync status
 */
export async function getOfflineStatus(): Promise<OfflineStatus> {
  return http.get<OfflineStatus>(`${BASE_PATH}/status`);
}

/**
 * Get queue statistics
 */
export async function getQueueStats(): Promise<QueueStats> {
  return http.get<QueueStats>(`${BASE_PATH}/queue/stats`);
}

/**
 * List queued operations
 */
export async function listQueuedOperations(status?: OperationStatus): Promise<QueuedOperation[]> {
  const params: Record<string, string> = {};
  if (status) params.status = status;
  return http.get<QueuedOperation[]>(`${BASE_PATH}/queue`, params);
}

/**
 * Retry a failed operation
 */
export async function retryOperation(operationId: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/queue/${operationId}/retry`, {});
}

/**
 * Cancel a pending operation
 */
export async function cancelOperation(operationId: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/queue/${operationId}/cancel`, {});
}

/**
 * Retry all failed operations
 */
export async function retryAllFailed(): Promise<void> {
  return http.post<void>(`${BASE_PATH}/queue/retry-all`, {});
}

/**
 * Clear completed operations
 */
export async function clearCompleted(): Promise<void> {
  return http.post<void>(`${BASE_PATH}/queue/clear`, {});
}

/**
 * Trigger manual sync
 */
export async function triggerSync(): Promise<void> {
  return http.post<void>(`${BASE_PATH}/sync`, {});
}

/**
 * Set sync mode
 */
export async function setSyncMode(mode: SyncMode): Promise<void> {
  return http.put<void>(`${BASE_PATH}/mode`, { mode });
}

/**
 * List pending changes
 */
export async function listPendingChanges(shareId?: string): Promise<LocalFileState[]> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  return http.get<LocalFileState[]>(`${BASE_PATH}/changes`, params);
}

/**
 * List pinned files
 */
export async function listPinnedFiles(shareId?: string): Promise<LocalFileState[]> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  return http.get<LocalFileState[]>(`${BASE_PATH}/pinned`, params);
}

/**
 * Pin a file for offline access
 */
export async function pinFile(shareId: string, path: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/pin`, { share_id: shareId, path });
}

/**
 * Unpin a file
 */
export async function unpinFile(shareId: string, path: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/unpin`, { share_id: shareId, path });
}

/**
 * List sync conflicts
 */
export async function listSyncConflicts(shareId?: string, unresolvedOnly = true): Promise<SyncConflict[]> {
  const params: Record<string, string> = {
    unresolved_only: unresolvedOnly.toString(),
  };
  if (shareId) params.share_id = shareId;
  return http.get<SyncConflict[]>(`${BASE_PATH}/conflicts`, params);
}

/**
 * Resolve a sync conflict
 */
export async function resolveSyncConflict(
  conflictId: string,
  resolution: SyncConflict['resolution']
): Promise<void> {
  return http.put<void>(`${BASE_PATH}/conflicts/${conflictId}`, { resolution });
}

// Query keys
export const offlineKeys = {
  all: ['offline'] as const,
  status: () => [...offlineKeys.all, 'status'] as const,
  queue: () => [...offlineKeys.all, 'queue'] as const,
  queueStats: () => [...offlineKeys.all, 'queue', 'stats'] as const,
  changes: (shareId?: string) => [...offlineKeys.all, 'changes', shareId] as const,
  pinned: (shareId?: string) => [...offlineKeys.all, 'pinned', shareId] as const,
  conflicts: (shareId?: string) => [...offlineKeys.all, 'conflicts', shareId] as const,
};

