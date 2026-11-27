import http from '@/lib/nos-client';
import type {
  SystemVersion,
  UpdateChannel,
  UpdateCheckResponse,
  UpdateApplyRequest,
  UpdateProgress,
  UpdateSnapshot,
  RollbackRequest,
  ChannelChangeRequest,
} from './updates.types';

export const updatesApi = {
  // Version and channel
  getVersion: () => 
    http.get<SystemVersion>('/v1/updates/version'),
    
  getChannel: () => 
    http.get<{ channel: UpdateChannel }>('/v1/updates/channel'),
    
  setChannel: (request: ChannelChangeRequest) =>
    http.post<{ status: string; channel: UpdateChannel }>(
      '/v1/updates/channel',
      request
    ),

  // Update operations
  checkForUpdates: () =>
    http.get<UpdateCheckResponse>('/v1/updates/check'),
    
  applyUpdate: (request?: UpdateApplyRequest) =>
    http.post<{ status: string; message: string }>(
      '/v1/updates/apply',
      request || {}
    ),
    
  getProgress: () =>
    http.get<UpdateProgress>('/v1/updates/progress'),
    
  rollback: (request: RollbackRequest) =>
    http.post<{ status: string; message: string }>(
      '/v1/updates/rollback',
      request
    ),

  // Snapshots
  listSnapshots: () =>
    http.get<{ snapshots: UpdateSnapshot[]; total: number }>(
      '/v1/updates/snapshots'
    ),
    
  deleteSnapshot: (id: string) =>
    http.del<{ status: string; snapshot_id: string }>(
      `/v1/updates/snapshots/${id}`
    ),

  // Progress streaming (Server-Sent Events)
  streamProgress: (onProgress: (progress: UpdateProgress) => void) => {
    const eventSource = http.updates.streamProgress();
    
    eventSource.onmessage = (event) => {
      try {
        const progress = JSON.parse(event.data);
        onProgress(progress);
      } catch (error) {
        console.error('Failed to parse progress event:', error);
      }
    };
    
    eventSource.onerror = (error) => {
      console.error('Progress stream error:', error);
      eventSource.close();
    };
    
    return eventSource;
  },
};

// Helper functions
export const formatVersion = (version: SystemVersion): string => {
  const parts = [];
  if (version.nosd_version) parts.push(`nosd ${version.nosd_version}`);
  if (version.agent_version) parts.push(`agent ${version.agent_version}`);
  if (version.webui_version) parts.push(`web ${version.webui_version}`);
  return parts.join(', ') || 'Unknown';
};

export const getUpdateStateColor = (state: UpdateProgress['state']): string => {
  switch (state) {
    case 'checking':
    case 'downloading':
    case 'applying':
    case 'verifying':
      return 'text-ink';
    case 'success':
      return 'text-status-ok-text';
    case 'failed':
      return 'text-status-failed-text';
    case 'rolling_back':
    case 'rolled_back':
      return 'text-status-warning-text';
    default:
      return 'text-ink-2';
  }
};

export const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
};

export const formatDuration = (seconds: number): string => {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes}m`;
};
