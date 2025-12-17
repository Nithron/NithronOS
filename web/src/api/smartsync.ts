/**
 * Smart Sync (On-Demand Files) API client for NithronSync
 */

import http from '@/lib/nos-client';

const BASE_PATH = '/api/v1/sync/smartsync';

// Types
export type PlaceholderState = 'cloud' | 'hydrating' | 'local' | 'pinned';
export type HydrationPriority = 0 | 50 | 100 | 200;

export interface PlaceholderFile {
  share_id: string;
  path: string;
  name: string;
  size: number;
  hash: string;
  modified_at: string;
  state: PlaceholderState;
  is_pinned: boolean;
  last_accessed?: string;
  hydration_progress?: number;
}

export interface DehydrationPolicy {
  enabled: boolean;
  max_local_size: number;
  max_file_age: number; // nanoseconds
  min_free_space: number;
  exclude_patterns: string[];
  pinned_always_local: boolean;
}

export interface SmartSyncStats {
  total_files: number;
  total_size: number;
  cloud_only_files: number;
  cloud_only_size: number;
  local_files: number;
  local_size: number;
  pinned_files: number;
  pinned_size: number;
  hydrating_files: number;
  queue_length: number;
}

export interface SmartSyncStatus {
  enabled: boolean;
  total_files: number;
  cloud_only: number;
  local_files: number;
  pinned_files: number;
  hydrating_files: number;
  queue_length: number;
  local_size: number;
  cloud_size: number;
}

// API Functions

/**
 * Get smart sync status
 */
export async function getSmartSyncStatus(): Promise<SmartSyncStatus> {
  return http.get<SmartSyncStatus>(`${BASE_PATH}/status`);
}

/**
 * Get detailed statistics
 */
export async function getSmartSyncStats(): Promise<SmartSyncStats> {
  return http.get<SmartSyncStats>(`${BASE_PATH}/stats`);
}

/**
 * Get dehydration policy
 */
export async function getPolicy(): Promise<DehydrationPolicy> {
  return http.get<DehydrationPolicy>(`${BASE_PATH}/policy`);
}

/**
 * Update dehydration policy
 */
export async function updatePolicy(policy: Partial<DehydrationPolicy>): Promise<DehydrationPolicy> {
  return http.put<DehydrationPolicy>(`${BASE_PATH}/policy`, policy);
}

/**
 * List placeholder files
 */
export async function listPlaceholders(shareId?: string): Promise<PlaceholderFile[]> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  return http.get<PlaceholderFile[]>(`${BASE_PATH}/placeholders`, params);
}

/**
 * List placeholders for a specific share
 */
export async function listSharePlaceholders(shareId: string): Promise<PlaceholderFile[]> {
  return http.get<PlaceholderFile[]>(`${BASE_PATH}/placeholders/${shareId}`);
}

/**
 * Get a specific placeholder
 */
export async function getPlaceholder(shareId: string, path: string): Promise<PlaceholderFile> {
  return http.get<PlaceholderFile>(`${BASE_PATH}/placeholders/${shareId}/${encodeURIComponent(path)}`);
}

/**
 * Request file hydration (download)
 */
export async function requestHydration(
  shareId: string,
  path: string,
  priority: HydrationPriority = 50
): Promise<void> {
  return http.post<void>(`${BASE_PATH}/hydrate`, {
    share_id: shareId,
    path,
    priority,
  });
}

/**
 * Cancel hydration request
 */
export async function cancelHydration(shareId: string, path: string): Promise<void> {
  return http.del<void>(`${BASE_PATH}/hydrate/${shareId}/${encodeURIComponent(path)}`);
}

/**
 * Get hydration queue status
 */
export async function getHydrationQueue(): Promise<{ queue_length: number; hydrating_files: number }> {
  return http.get<{ queue_length: number; hydrating_files: number }>(`${BASE_PATH}/hydration-queue`);
}

/**
 * Pin a file (always keep local)
 */
export async function pinFile(shareId: string, path: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/pin`, { share_id: shareId, path });
}

/**
 * Unpin a file
 */
export async function unpinFile(shareId: string, path: string): Promise<void> {
  return http.del<void>(`${BASE_PATH}/pin/${shareId}/${encodeURIComponent(path)}`);
}

/**
 * List pinned files
 */
export async function listPinnedFiles(shareId?: string): Promise<{ pinned_count: number; pinned_size: number }> {
  const params: Record<string, string> = {};
  if (shareId) params.share_id = shareId;
  return http.get<{ pinned_count: number; pinned_size: number }>(`${BASE_PATH}/pinned`, params);
}

/**
 * Dehydrate a file (convert to placeholder)
 */
export async function dehydrateFile(shareId: string, path: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/dehydrate`, { share_id: shareId, path });
}

/**
 * List cloud-only files
 */
export async function listCloudOnlyFiles(shareId: string): Promise<PlaceholderFile[]> {
  return http.get<PlaceholderFile[]>(`${BASE_PATH}/cloud-only`, { share_id: shareId });
}

/**
 * List local files
 */
export async function listLocalFiles(shareId: string): Promise<PlaceholderFile[]> {
  return http.get<PlaceholderFile[]>(`${BASE_PATH}/local`, { share_id: shareId });
}

// Utility functions

/**
 * Format file size for display
 */
export function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * Format duration (nanoseconds to human readable)
 */
export function formatDuration(ns: number): string {
  const days = ns / (24 * 60 * 60 * 1e9);
  if (days >= 1) return `${Math.round(days)} days`;
  const hours = ns / (60 * 60 * 1e9);
  if (hours >= 1) return `${Math.round(hours)} hours`;
  const minutes = ns / (60 * 1e9);
  return `${Math.round(minutes)} minutes`;
}

/**
 * Get state display info
 */
export function getStateInfo(state: PlaceholderState): { label: string; color: string; icon: string } {
  switch (state) {
    case 'cloud':
      return { label: 'Cloud Only', color: 'blue', icon: 'cloud' };
    case 'hydrating':
      return { label: 'Downloading', color: 'yellow', icon: 'download' };
    case 'local':
      return { label: 'Available', color: 'green', icon: 'check' };
    case 'pinned':
      return { label: 'Pinned', color: 'purple', icon: 'pin' };
  }
}

// Query keys
export const smartSyncKeys = {
  all: ['smartsync'] as const,
  status: () => [...smartSyncKeys.all, 'status'] as const,
  stats: () => [...smartSyncKeys.all, 'stats'] as const,
  policy: () => [...smartSyncKeys.all, 'policy'] as const,
  placeholders: (shareId?: string) => [...smartSyncKeys.all, 'placeholders', shareId] as const,
  placeholder: (shareId: string, path: string) => [...smartSyncKeys.all, 'placeholder', shareId, path] as const,
  hydrationQueue: () => [...smartSyncKeys.all, 'hydration-queue'] as const,
  pinned: (shareId?: string) => [...smartSyncKeys.all, 'pinned', shareId] as const,
  cloudOnly: (shareId: string) => [...smartSyncKeys.all, 'cloud-only', shareId] as const,
  local: (shareId: string) => [...smartSyncKeys.all, 'local', shareId] as const,
};

