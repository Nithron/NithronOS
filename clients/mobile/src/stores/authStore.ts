/**
 * Authentication State Store
 * Manages device registration and authentication state
 */

import {create} from 'zustand';
import {apiClient} from '../api/client';
import {Device, DeviceRegistration, ServerInfo} from '../api/types';

interface AuthState {
  // State
  isInitialized: boolean;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  serverInfo: ServerInfo | null;
  currentDevice: Device | null;

  // Actions
  initialize: () => Promise<void>;
  connectToServer: (url: string) => Promise<ServerInfo>;
  registerDevice: (registration: DeviceRegistration) => Promise<void>;
  registerFromQRCode: (qrData: string) => Promise<void>;
  logout: () => Promise<void>;
  clearError: () => void;
  refreshDeviceInfo: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  // Initial state
  isInitialized: false,
  isAuthenticated: false,
  isLoading: false,
  error: null,
  serverInfo: null,
  currentDevice: null,

  // Initialize from stored credentials
  initialize: async () => {
    try {
      set({isLoading: true, error: null});
      
      const hasCredentials = await apiClient.initialize();
      
      if (hasCredentials) {
        // Verify credentials by fetching device info
        try {
          const deviceId = apiClient.getDeviceId();
          const device = await apiClient.getDevice(deviceId);
          set({
            isAuthenticated: true,
            currentDevice: device,
            serverInfo: {
              url: apiClient.getServerUrl(),
              name: 'NithronOS',
              version: 'unknown',
              features: [],
            },
          });
        } catch (error) {
          // Credentials invalid, need to re-authenticate
          await apiClient.logout();
          set({isAuthenticated: false, currentDevice: null});
        }
      }
    } catch (error) {
      console.error('Auth initialization error:', error);
      set({error: (error as Error).message});
    } finally {
      set({isInitialized: true, isLoading: false});
    }
  },

  // Connect to a server
  connectToServer: async (url: string) => {
    set({isLoading: true, error: null});
    try {
      const serverInfo = await apiClient.setServer(url);
      set({serverInfo});
      return serverInfo;
    } catch (error) {
      const message = (error as Error).message || 'Failed to connect to server';
      set({error: message});
      throw error;
    } finally {
      set({isLoading: false});
    }
  },

  // Register device with username/password or manual registration
  registerDevice: async (registration: DeviceRegistration) => {
    set({isLoading: true, error: null});
    try {
      const tokenResponse = await apiClient.registerDevice(registration);
      const device = await apiClient.getDevice(tokenResponse.device_id);
      set({
        isAuthenticated: true,
        currentDevice: device,
      });
    } catch (error) {
      const message = (error as Error).message || 'Failed to register device';
      set({error: message});
      throw error;
    } finally {
      set({isLoading: false});
    }
  },

  // Register device using QR code
  registerFromQRCode: async (qrData: string) => {
    set({isLoading: true, error: null});
    try {
      const tokenResponse = await apiClient.registerFromQRCode(qrData);
      
      // If we got a device_id, fetch full device info
      if (tokenResponse.device_id) {
        try {
          const device = await apiClient.getDevice(tokenResponse.device_id);
          set({currentDevice: device});
        } catch {
          // Device info fetch failed, use basic info
          set({
            currentDevice: {
              id: tokenResponse.device_id,
              name: 'Mobile Device',
              type: 'mobile',
              platform: 'android',
              created_at: new Date().toISOString(),
              last_seen: new Date().toISOString(),
              is_online: true,
            },
          });
        }
      }
      
      set({
        isAuthenticated: true,
        serverInfo: {
          url: apiClient.getServerUrl(),
          name: 'NithronOS',
          version: 'unknown',
          features: [],
        },
      });
    } catch (error) {
      const message = (error as Error).message || 'Failed to register from QR code';
      set({error: message});
      throw error;
    } finally {
      set({isLoading: false});
    }
  },

  // Logout
  logout: async () => {
    set({isLoading: true});
    try {
      await apiClient.logout();
      set({
        isAuthenticated: false,
        currentDevice: null,
        serverInfo: null,
      });
    } finally {
      set({isLoading: false});
    }
  },

  // Clear error
  clearError: () => {
    set({error: null});
  },

  // Refresh device info
  refreshDeviceInfo: async () => {
    const deviceId = apiClient.getDeviceId();
    if (!deviceId) return;

    try {
      const device = await apiClient.getDevice(deviceId);
      set({currentDevice: device});
    } catch (error) {
      console.error('Failed to refresh device info:', error);
    }
  },
}));

export default useAuthStore;

