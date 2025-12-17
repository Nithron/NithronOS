/**
 * Encryption API client for NithronSync
 */

import http from '@/lib/nos-client';

const BASE_PATH = '/api/v1/sync/encryption';

// Types
export interface EncryptionStatus {
  enabled: boolean;
  algorithm: 'aes-256-gcm' | 'chacha20-poly1305' | 'xchacha20-poly1305';
  master_key_initialized: boolean;
  recovery_key_exists: boolean;
  shares_encrypted: number;
  total_encrypted_files: number;
  total_encrypted_size: number;
}

export interface ShareEncryptionStatus {
  share_id: string;
  share_name: string;
  encrypted: boolean;
  encryption_enabled_at?: string;
  total_files: number;
  encrypted_files: number;
  encryption_progress: number;
}

export interface KeyInfo {
  id: string;
  type: 'master' | 'share' | 'device' | 'recovery';
  created_at: string;
  expires_at?: string;
  algorithm: string;
}

export interface EncryptionSettings {
  default_algorithm: 'aes-256-gcm' | 'chacha20-poly1305' | 'xchacha20-poly1305';
  encrypt_new_shares: boolean;
  encrypt_filenames: boolean;
  key_rotation_days: number;
}

// API Functions

/**
 * Get encryption status
 */
export async function getEncryptionStatus(): Promise<EncryptionStatus> {
  return http.get<EncryptionStatus>(`${BASE_PATH}/status`);
}

/**
 * Initialize encryption with a password
 */
export async function initializeEncryption(password: string): Promise<{ recovery_key: string }> {
  return http.post<{ recovery_key: string }>(`${BASE_PATH}/initialize`, { password });
}

/**
 * Unlock encryption with password
 */
export async function unlockEncryption(password: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/unlock`, { password });
}

/**
 * Lock encryption (require password to access encrypted data)
 */
export async function lockEncryption(): Promise<void> {
  return http.post<void>(`${BASE_PATH}/lock`, {});
}

/**
 * Change encryption password
 */
export async function changeEncryptionPassword(oldPassword: string, newPassword: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/change-password`, {
    old_password: oldPassword,
    new_password: newPassword,
  });
}

/**
 * Generate a new recovery key
 */
export async function generateRecoveryKey(): Promise<{ recovery_key: string }> {
  return http.post<{ recovery_key: string }>(`${BASE_PATH}/recovery-key`, {});
}

/**
 * Recover encryption using recovery key
 */
export async function recoverWithKey(recoveryKey: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/recover`, { recovery_key: recoveryKey });
}

/**
 * Get share encryption status
 */
export async function getShareEncryptionStatus(shareId: string): Promise<ShareEncryptionStatus> {
  return http.get<ShareEncryptionStatus>(`${BASE_PATH}/shares/${shareId}`);
}

/**
 * Enable encryption for a share
 */
export async function enableShareEncryption(shareId: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/shares/${shareId}/enable`, {});
}

/**
 * Disable encryption for a share
 */
export async function disableShareEncryption(shareId: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/shares/${shareId}/disable`, {});
}

/**
 * Get encryption settings
 */
export async function getEncryptionSettings(): Promise<EncryptionSettings> {
  return http.get<EncryptionSettings>(`${BASE_PATH}/settings`);
}

/**
 * Update encryption settings
 */
export async function updateEncryptionSettings(settings: Partial<EncryptionSettings>): Promise<EncryptionSettings> {
  return http.put<EncryptionSettings>(`${BASE_PATH}/settings`, settings);
}

/**
 * List encryption keys
 */
export async function listKeys(): Promise<KeyInfo[]> {
  return http.get<KeyInfo[]>(`${BASE_PATH}/keys`);
}

/**
 * Rotate share encryption key
 */
export async function rotateShareKey(shareId: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/shares/${shareId}/rotate-key`, {});
}

/**
 * Export device public key for sharing
 */
export async function exportPublicKey(deviceId: string): Promise<{ public_key: string }> {
  return http.get<{ public_key: string }>(`${BASE_PATH}/devices/${deviceId}/public-key`);
}

/**
 * Import a device's public key
 */
export async function importPublicKey(deviceId: string, publicKey: string): Promise<void> {
  return http.post<void>(`${BASE_PATH}/devices/${deviceId}/public-key`, { public_key: publicKey });
}

// Query keys
export const encryptionKeys = {
  all: ['encryption'] as const,
  status: () => [...encryptionKeys.all, 'status'] as const,
  settings: () => [...encryptionKeys.all, 'settings'] as const,
  keys: () => [...encryptionKeys.all, 'keys'] as const,
  share: (shareId: string) => [...encryptionKeys.all, 'share', shareId] as const,
};

