/**
 * What the app shows from nosd's sync API: the shares this device can
 * reach, and this device's sync activity and conflicts.
 *
 * The app browses shares; it doesn't sync files itself (yet), so activity
 * and conflicts list what nosd recorded for this device.
 */

import {create} from 'zustand';

import {apiClient} from '../api/client';
import {
  ConflictResolution,
  SyncActivity,
  SyncConflict,
  SyncShare,
} from '../api/types';
import {errorMessage} from './authStore';

const ACTIVITY_PAGE_SIZE = 50;

interface SyncStoreState {
  shares: SyncShare[];
  sharesError: string | null;

  activities: SyncActivity[];
  activityTotal: number;
  activityPage: number;
  activityError: string | null;

  conflicts: SyncConflict[];
  conflictsError: string | null;

  refreshShares: () => Promise<void>;
  refreshActivity: () => Promise<void>;
  loadMoreActivity: () => Promise<void>;
  refreshConflicts: () => Promise<void>;
  resolveConflict: (
    id: string,
    resolution: ConflictResolution,
  ) => Promise<void>;
  reset: () => void;
}

const EMPTY = {
  shares: [],
  sharesError: null,
  activities: [],
  activityTotal: 0,
  activityPage: 0,
  activityError: null,
  conflicts: [],
  conflictsError: null,
};

export const useSyncStore = create<SyncStoreState>((set, get) => ({
  ...EMPTY,

  refreshShares: async () => {
    try {
      set({shares: await apiClient.listShares(), sharesError: null});
    } catch (error) {
      set({sharesError: errorMessage(error)});
    }
  },

  refreshActivity: async () => {
    try {
      const page = await apiClient.activityPage(1, ACTIVITY_PAGE_SIZE);
      set({
        activities: page.activities ?? [],
        activityTotal: page.total,
        activityPage: 1,
        activityError: null,
      });
    } catch (error) {
      set({activityError: errorMessage(error)});
    }
  },

  loadMoreActivity: async () => {
    const {activities, activityTotal, activityPage} = get();
    if (activities.length >= activityTotal) {
      return;
    }
    try {
      const next = activityPage + 1;
      const page = await apiClient.activityPage(next, ACTIVITY_PAGE_SIZE);
      set({
        activities: [...activities, ...(page.activities ?? [])],
        activityTotal: page.total,
        activityPage: next,
        activityError: null,
      });
    } catch (error) {
      set({activityError: errorMessage(error)});
    }
  },

  refreshConflicts: async () => {
    try {
      set({conflicts: await apiClient.listConflicts(), conflictsError: null});
    } catch (error) {
      set({conflictsError: errorMessage(error)});
    }
  },

  resolveConflict: async (id, resolution) => {
    const resolved = await apiClient.resolveConflict(id, resolution);
    set(state => ({
      conflicts: state.conflicts.map(c => (c.id === id ? resolved : c)),
    }));
  },

  reset: () => set(EMPTY),
}));

/** Conflicts still waiting for a decision. */
export function unresolvedCount(conflicts: SyncConflict[]): number {
  return conflicts.filter(c => !c.resolved).length;
}

export default useSyncStore;
