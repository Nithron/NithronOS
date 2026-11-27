/**
 * The pairing: whether this phone is paired with a nOS machine, and the
 * device record nosd keeps for it.
 */

import {create} from 'zustand';

import {apiClient, ApiError} from '../api/client';
import {parsePairingLink} from '../api/pairing';
import {Device} from '../api/types';

export type PairingStatus = 'starting' | 'unpaired' | 'paired';

interface AuthState {
  status: PairingStatus;
  serverUrl: string | null;
  /** This device as nosd sees it; null until it has been fetched */
  device: Device | null;
  isBusy: boolean;
  error: string | null;

  initialize: () => Promise<void>;
  /** Pairs from the link the web interface shows (or its QR code). */
  pairWithLink: (link: string) => Promise<void>;
  refreshDevice: () => Promise<void>;
  renameDevice: (name: string) => Promise<void>;
  /** Removes the device on the server (when it can) and forgets the pairing. */
  unpair: () => Promise<{revoked: boolean}>;
  clearError: () => void;
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export const useAuthStore = create<AuthState>((set, get) => {
  apiClient.onSessionEnded = () =>
    set({status: 'unpaired', serverUrl: null, device: null});

  async function fetchDevice(): Promise<Device> {
    const config = await apiClient.getConfig();
    return apiClient.getDevice(config.device_id);
  }

  return {
    status: 'starting',
    serverUrl: null,
    device: null,
    isBusy: false,
    error: null,

    initialize: async () => {
      try {
        const paired = await apiClient.restore();
        set({
          status: paired ? 'paired' : 'unpaired',
          serverUrl: apiClient.serverUrl,
        });
      } catch (error) {
        set({status: 'unpaired', error: errorMessage(error)});
        return;
      }
      if (get().status === 'paired') {
        // The app opens offline too; the device record comes when it can.
        await get().refreshDevice();
      }
    },

    pairWithLink: async (link: string) => {
      set({isBusy: true, error: null});
      try {
        const pairing = parsePairingLink(link);
        const config = await apiClient.pair(pairing);
        const device = await apiClient.getDevice(config.device_id);
        set({status: 'paired', serverUrl: apiClient.serverUrl, device});
      } catch (error) {
        set({error: errorMessage(error)});
        throw error;
      } finally {
        set({isBusy: false});
      }
    },

    refreshDevice: async () => {
      try {
        set({device: await fetchDevice(), error: null});
      } catch (error) {
        if (get().status === 'paired') {
          set({error: errorMessage(error)});
        }
      }
    },

    renameDevice: async (name: string) => {
      const {device} = get();
      if (!device) {
        throw new Error("This device's record hasn't loaded yet");
      }
      set({device: await apiClient.renameDevice(device.id, name)});
    },

    unpair: async () => {
      set({isBusy: true, error: null});
      let revoked = false;
      try {
        const id = get().device?.id ?? (await apiClient.getConfig()).device_id;
        await apiClient.revokeDevice(id);
        revoked = true;
      } catch (error) {
        // Offline, or already removed: forget the pairing here anyway.
        revoked = error instanceof ApiError && error.status === 404;
      }
      try {
        await apiClient.forget();
        set({status: 'unpaired', serverUrl: null, device: null});
      } finally {
        set({isBusy: false});
      }
      return {revoked};
    },

    clearError: () => set({error: null}),
  };
});

export default useAuthStore;
