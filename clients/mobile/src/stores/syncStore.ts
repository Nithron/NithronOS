/**
 * Sync State Store
 * Manages file synchronization state and operations
 */

import {create} from 'zustand';
import {apiClient} from '../api/client';
import {
  SyncShare,
  SyncConfig,
  SyncState,
  FileMetadata,
  SyncConflict,
  SyncActivity,
  ChangesResponse,
  TransferProgress,
} from '../api/types';

// Sync status enum
export type SyncStatus = 'idle' | 'syncing' | 'paused' | 'error' | 'offline';

interface SyncStoreState {
  // State
  isInitialized: boolean;
  status: SyncStatus;
  shares: SyncShare[];
  config: SyncConfig | null;
  syncStates: Record<string, SyncState>;
  conflicts: SyncConflict[];
  activities: SyncActivity[];
  transfers: TransferProgress[];
  error: string | null;

  // Current navigation state
  currentShare: SyncShare | null;
  currentPath: string;
  currentFiles: FileMetadata[];
  isLoadingFiles: boolean;

  // Overall stats
  totalPendingUploads: number;
  totalPendingDownloads: number;
  totalConflicts: number;

  // Actions
  initialize: () => Promise<void>;
  refreshShares: () => Promise<void>;
  refreshConfig: () => Promise<void>;
  updateConfig: (config: Partial<SyncConfig>) => Promise<void>;
  syncNow: () => Promise<void>;
  pauseSync: () => void;
  resumeSync: () => void;

  // File browsing
  setCurrentShare: (share: SyncShare | null) => void;
  navigateToPath: (path: string) => Promise<void>;
  refreshCurrentDirectory: () => Promise<void>;

  // Conflict resolution
  refreshConflicts: () => Promise<void>;
  resolveConflict: (
    conflictId: string,
    resolution: 'keep_local' | 'keep_remote' | 'keep_both'
  ) => Promise<void>;

  // Activity
  refreshActivity: () => Promise<void>;
  loadMoreActivity: () => Promise<void>;

  // Sync state per share
  getSyncState: (shareId: string) => Promise<SyncState | null>;
  getChanges: (shareId: string) => Promise<ChangesResponse | null>;
}

export const useSyncStore = create<SyncStoreState>((set, get) => ({
  // Initial state
  isInitialized: false,
  status: 'idle',
  shares: [],
  config: null,
  syncStates: {},
  conflicts: [],
  activities: [],
  transfers: [],
  error: null,

  currentShare: null,
  currentPath: '/',
  currentFiles: [],
  isLoadingFiles: false,

  totalPendingUploads: 0,
  totalPendingDownloads: 0,
  totalConflicts: 0,

  // Initialize sync store
  initialize: async () => {
    if (!apiClient.isAuthenticated()) {
      return;
    }

    try {
      // Load initial data in parallel
      const [shares, config, conflicts, activities] = await Promise.all([
        apiClient.listShares().catch(() => []),
        apiClient.getSyncConfig().catch(() => null),
        apiClient.listConflicts().catch(() => []),
        apiClient.getRecentActivity(20).catch(() => []),
      ]);

      // Load sync states for each share
      const syncStates: Record<string, SyncState> = {};
      for (const share of shares) {
        try {
          const state = await apiClient.getSyncState(share.share_id);
          syncStates[share.share_id] = state;
        } catch {
          // Ignore errors for individual shares
        }
      }

      // Calculate totals
      let totalPendingUploads = 0;
      let totalPendingDownloads = 0;
      for (const state of Object.values(syncStates)) {
        totalPendingUploads += state.pending_uploads || 0;
        totalPendingDownloads += state.pending_downloads || 0;
      }

      set({
        isInitialized: true,
        shares,
        config,
        conflicts,
        activities,
        syncStates,
        totalPendingUploads,
        totalPendingDownloads,
        totalConflicts: conflicts.filter(c => !c.resolved).length,
        status: config?.pause_sync ? 'paused' : 'idle',
      });
    } catch (error) {
      console.error('Sync store initialization error:', error);
      set({
        isInitialized: true,
        error: (error as Error).message,
        status: 'error',
      });
    }
  },

  // Refresh shares list
  refreshShares: async () => {
    try {
      const shares = await apiClient.listShares();
      set({shares});
    } catch (error) {
      console.error('Failed to refresh shares:', error);
    }
  },

  // Refresh config
  refreshConfig: async () => {
    try {
      const config = await apiClient.getSyncConfig();
      set({config});
    } catch (error) {
      console.error('Failed to refresh config:', error);
    }
  },

  // Update config
  updateConfig: async (configUpdate: Partial<SyncConfig>) => {
    try {
      const config = await apiClient.updateSyncConfig(configUpdate);
      set({config});
    } catch (error) {
      console.error('Failed to update config:', error);
      throw error;
    }
  },

  // Trigger immediate sync
  syncNow: async () => {
    const {status, shares} = get();
    if (status === 'syncing') return;

    set({status: 'syncing', error: null});

    try {
      // Process each share
      for (const share of shares) {
        try {
          // Get changes since last sync
          const state = get().syncStates[share.share_id];
          const cursor = state?.cursor;
          
          const changes = await apiClient.getChanges(share.share_id, cursor);
          
          // Process changes (simplified - real implementation would handle file transfers)
          if (changes.changes.length > 0) {
            console.log(`Processing ${changes.changes.length} changes for ${share.name}`);
            
            // Update sync state with new cursor
            const newState = await apiClient.updateSyncState(share.share_id, {
              cursor: changes.cursor,
              last_sync: new Date().toISOString(),
            });
            
            set(state => ({
              syncStates: {
                ...state.syncStates,
                [share.share_id]: newState,
              },
            }));
          }
        } catch (error) {
          console.error(`Sync error for share ${share.name}:`, error);
        }
      }

      set({status: 'idle'});
    } catch (error) {
      set({
        status: 'error',
        error: (error as Error).message,
      });
    }
  },

  // Pause sync
  pauseSync: () => {
    set({status: 'paused'});
    // Also update server config
    apiClient.updateSyncConfig({pause_sync: true}).catch(console.error);
  },

  // Resume sync
  resumeSync: () => {
    set({status: 'idle'});
    apiClient.updateSyncConfig({pause_sync: false}).catch(console.error);
    // Trigger immediate sync
    get().syncNow();
  },

  // Set current share for browsing
  setCurrentShare: (share: SyncShare | null) => {
    set({
      currentShare: share,
      currentPath: '/',
      currentFiles: [],
    });
    if (share) {
      get().navigateToPath('/');
    }
  },

  // Navigate to a path within current share
  navigateToPath: async (path: string) => {
    const {currentShare} = get();
    if (!currentShare) return;

    set({isLoadingFiles: true, currentPath: path});

    try {
      const files = await apiClient.listDirectory(currentShare.share_id, path);
      set({currentFiles: files, isLoadingFiles: false});
    } catch (error) {
      console.error('Failed to load directory:', error);
      set({
        isLoadingFiles: false,
        currentFiles: [],
        error: (error as Error).message,
      });
    }
  },

  // Refresh current directory
  refreshCurrentDirectory: async () => {
    const {currentPath} = get();
    await get().navigateToPath(currentPath);
  },

  // Refresh conflicts
  refreshConflicts: async () => {
    try {
      const conflicts = await apiClient.listConflicts();
      set({
        conflicts,
        totalConflicts: conflicts.filter(c => !c.resolved).length,
      });
    } catch (error) {
      console.error('Failed to refresh conflicts:', error);
    }
  },

  // Resolve a conflict
  resolveConflict: async (conflictId, resolution) => {
    try {
      await apiClient.resolveConflict(conflictId, resolution);
      await get().refreshConflicts();
    } catch (error) {
      console.error('Failed to resolve conflict:', error);
      throw error;
    }
  },

  // Refresh activity
  refreshActivity: async () => {
    try {
      const activities = await apiClient.getRecentActivity(20);
      set({activities});
    } catch (error) {
      console.error('Failed to refresh activity:', error);
    }
  },

  // Load more activity (pagination)
  loadMoreActivity: async () => {
    const {activities} = get();
    const page = Math.floor(activities.length / 50) + 1;

    try {
      const response = await apiClient.getActivityHistory(page, 50);
      set({
        activities: [...activities, ...response.activities],
      });
    } catch (error) {
      console.error('Failed to load more activity:', error);
    }
  },

  // Get sync state for a share
  getSyncState: async (shareId: string) => {
    try {
      const state = await apiClient.getSyncState(shareId);
      set(currentState => ({
        syncStates: {
          ...currentState.syncStates,
          [shareId]: state,
        },
      }));
      return state;
    } catch (error) {
      console.error('Failed to get sync state:', error);
      return null;
    }
  },

  // Get changes for a share
  getChanges: async (shareId: string) => {
    const state = get().syncStates[shareId];
    try {
      return await apiClient.getChanges(shareId, state?.cursor);
    } catch (error) {
      console.error('Failed to get changes:', error);
      return null;
    }
  },
}));

export default useSyncStore;

