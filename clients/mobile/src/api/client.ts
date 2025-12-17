/**
 * NithronSync API Client
 * Handles all communication with the NithronOS backend
 */

import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  DeviceRegistration,
  DeviceTokenResponse,
  RefreshTokenResponse,
  Device,
  SyncShare,
  SyncConfig,
  FileMetadata,
  ChangesResponse,
  SyncState,
  SyncConflict,
  SyncActivity,
  ActivityListResponse,
  BlockHashRequest,
  BlockHashResponse,
  APIError,
  ServerInfo,
} from './types';

// Storage keys
const STORAGE_KEYS = {
  SERVER_URL: '@nithron_server_url',
  DEVICE_ID: '@nithron_device_id',
  DEVICE_TOKEN: '@nithron_device_token',
  REFRESH_TOKEN: '@nithron_refresh_token',
  TOKEN_EXPIRES: '@nithron_token_expires',
};

class NithronAPIClient {
  private baseUrl: string = '';
  private deviceToken: string = '';
  private refreshToken: string = '';
  private tokenExpires: Date | null = null;
  private deviceId: string = '';
  private isRefreshing: boolean = false;
  private refreshPromise: Promise<void> | null = null;

  /**
   * Initialize client from stored credentials
   */
  async initialize(): Promise<boolean> {
    try {
      const [serverUrl, deviceId, deviceToken, refreshToken, tokenExpires] =
        await Promise.all([
          AsyncStorage.getItem(STORAGE_KEYS.SERVER_URL),
          AsyncStorage.getItem(STORAGE_KEYS.DEVICE_ID),
          AsyncStorage.getItem(STORAGE_KEYS.DEVICE_TOKEN),
          AsyncStorage.getItem(STORAGE_KEYS.REFRESH_TOKEN),
          AsyncStorage.getItem(STORAGE_KEYS.TOKEN_EXPIRES),
        ]);

      if (serverUrl && deviceId && deviceToken && refreshToken) {
        this.baseUrl = serverUrl;
        this.deviceId = deviceId;
        this.deviceToken = deviceToken;
        this.refreshToken = refreshToken;
        this.tokenExpires = tokenExpires ? new Date(tokenExpires) : null;
        return true;
      }
      return false;
    } catch (error) {
      console.error('Failed to initialize API client:', error);
      return false;
    }
  }

  /**
   * Configure client with server URL
   */
  async setServer(url: string): Promise<ServerInfo> {
    // Normalize URL
    const normalizedUrl = url.replace(/\/+$/, '');
    
    // Test connection and get server info
    const response = await fetch(`${normalizedUrl}/api/v1/health`);
    if (!response.ok) {
      throw new Error('Failed to connect to server');
    }

    const healthData = await response.json();
    this.baseUrl = normalizedUrl;
    await AsyncStorage.setItem(STORAGE_KEYS.SERVER_URL, normalizedUrl);

    return {
      url: normalizedUrl,
      name: healthData.name || 'NithronOS',
      version: healthData.version || 'unknown',
      features: healthData.features || [],
    };
  }

  /**
   * Register this device with the server
   */
  async registerDevice(registration: DeviceRegistration): Promise<DeviceTokenResponse> {
    const response = await this.request<DeviceTokenResponse>(
      '/api/v1/sync/devices/register',
      {
        method: 'POST',
        body: JSON.stringify(registration),
      },
      false // Don't use auth for registration
    );

    // Store credentials
    this.deviceId = response.device_id;
    this.deviceToken = response.device_token;
    this.refreshToken = response.refresh_token;
    this.tokenExpires = new Date(response.expires_at);

    await Promise.all([
      AsyncStorage.setItem(STORAGE_KEYS.DEVICE_ID, response.device_id),
      AsyncStorage.setItem(STORAGE_KEYS.DEVICE_TOKEN, response.device_token),
      AsyncStorage.setItem(STORAGE_KEYS.REFRESH_TOKEN, response.refresh_token),
      AsyncStorage.setItem(STORAGE_KEYS.TOKEN_EXPIRES, response.expires_at),
    ]);

    return response;
  }

  /**
   * Register device using QR code data
   */
  async registerFromQRCode(qrData: string): Promise<DeviceTokenResponse> {
    // Parse QR code URL: nithronos://sync?server=...&token=...
    const url = new URL(qrData);
    
    if (url.protocol !== 'nithronos:') {
      throw new Error('Invalid QR code format');
    }

    const serverUrl = url.searchParams.get('server');
    const token = url.searchParams.get('token');
    const deviceName = url.searchParams.get('name');

    if (!serverUrl) {
      throw new Error('Server URL not found in QR code');
    }

    // Connect to server
    await this.setServer(serverUrl);

    if (token) {
      // Token provided directly - use it
      const response: DeviceTokenResponse = {
        device_id: url.searchParams.get('device_id') || '',
        device_token: token,
        refresh_token: url.searchParams.get('refresh_token') || '',
        expires_at: url.searchParams.get('expires') || new Date(Date.now() + 86400000).toISOString(),
        scopes: ['sync.read', 'sync.write'],
      };

      this.deviceId = response.device_id;
      this.deviceToken = response.device_token;
      this.refreshToken = response.refresh_token;
      this.tokenExpires = new Date(response.expires_at);

      await Promise.all([
        AsyncStorage.setItem(STORAGE_KEYS.DEVICE_ID, response.device_id),
        AsyncStorage.setItem(STORAGE_KEYS.DEVICE_TOKEN, response.device_token),
        AsyncStorage.setItem(STORAGE_KEYS.REFRESH_TOKEN, response.refresh_token),
        AsyncStorage.setItem(STORAGE_KEYS.TOKEN_EXPIRES, response.expires_at),
      ]);

      return response;
    } else {
      // Need to register device
      const platform = this.detectPlatform();
      return this.registerDevice({
        name: deviceName || `Mobile Device (${platform})`,
        type: 'mobile',
        platform,
        osVersion: this.getOSVersion(),
      });
    }
  }

  /**
   * Refresh the access token
   */
  async refreshAccessToken(): Promise<void> {
    // Prevent multiple simultaneous refresh attempts
    if (this.isRefreshing) {
      await this.refreshPromise;
      return;
    }

    this.isRefreshing = true;
    this.refreshPromise = this.doRefresh();

    try {
      await this.refreshPromise;
    } finally {
      this.isRefreshing = false;
      this.refreshPromise = null;
    }
  }

  private async doRefresh(): Promise<void> {
    const response = await fetch(`${this.baseUrl}/api/v1/sync/devices/refresh`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        device_id: this.deviceId,
        refresh_token: this.refreshToken,
      }),
    });

    if (!response.ok) {
      // Refresh failed - need to re-authenticate
      await this.logout();
      throw new Error('Session expired. Please reconnect.');
    }

    const data: RefreshTokenResponse = await response.json();
    this.deviceToken = data.device_token;
    this.tokenExpires = new Date(data.expires_at);

    await Promise.all([
      AsyncStorage.setItem(STORAGE_KEYS.DEVICE_TOKEN, data.device_token),
      AsyncStorage.setItem(STORAGE_KEYS.TOKEN_EXPIRES, data.expires_at),
    ]);
  }

  /**
   * Logout and clear credentials
   */
  async logout(): Promise<void> {
    this.deviceId = '';
    this.deviceToken = '';
    this.refreshToken = '';
    this.tokenExpires = null;

    await Promise.all([
      AsyncStorage.removeItem(STORAGE_KEYS.DEVICE_ID),
      AsyncStorage.removeItem(STORAGE_KEYS.DEVICE_TOKEN),
      AsyncStorage.removeItem(STORAGE_KEYS.REFRESH_TOKEN),
      AsyncStorage.removeItem(STORAGE_KEYS.TOKEN_EXPIRES),
    ]);
  }

  /**
   * Check if client is authenticated
   */
  isAuthenticated(): boolean {
    return !!this.deviceToken && !!this.baseUrl;
  }

  /**
   * Get device ID
   */
  getDeviceId(): string {
    return this.deviceId;
  }

  /**
   * Get server URL
   */
  getServerUrl(): string {
    return this.baseUrl;
  }

  // ==================== Device Management ====================

  /**
   * List all registered devices
   */
  async listDevices(): Promise<Device[]> {
    return this.request<Device[]>('/api/v1/sync/devices');
  }

  /**
   * Get device by ID
   */
  async getDevice(deviceId: string): Promise<Device> {
    return this.request<Device>(`/api/v1/sync/devices/${deviceId}`);
  }

  /**
   * Update device name
   */
  async updateDevice(deviceId: string, name: string): Promise<Device> {
    return this.request<Device>(`/api/v1/sync/devices/${deviceId}`, {
      method: 'PUT',
      body: JSON.stringify({name}),
    });
  }

  /**
   * Revoke a device
   */
  async revokeDevice(deviceId: string): Promise<void> {
    await this.request<void>(`/api/v1/sync/devices/${deviceId}`, {
      method: 'DELETE',
    });
  }

  // ==================== Sync Shares ====================

  /**
   * List sync-enabled shares
   */
  async listShares(): Promise<SyncShare[]> {
    return this.request<SyncShare[]>('/api/v1/sync/shares');
  }

  // ==================== Sync Configuration ====================

  /**
   * Get sync configuration for this device
   */
  async getSyncConfig(): Promise<SyncConfig> {
    return this.request<SyncConfig>('/api/v1/sync/config');
  }

  /**
   * Update sync configuration
   */
  async updateSyncConfig(config: Partial<SyncConfig>): Promise<SyncConfig> {
    return this.request<SyncConfig>('/api/v1/sync/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // ==================== File Operations ====================

  /**
   * Get file/folder metadata
   */
  async getMetadata(shareId: string, path: string): Promise<FileMetadata> {
    const encodedPath = encodeURIComponent(path);
    return this.request<FileMetadata>(
      `/api/v1/sync/shares/${shareId}/metadata?path=${encodedPath}`
    );
  }

  /**
   * List directory contents
   */
  async listDirectory(shareId: string, path: string = '/'): Promise<FileMetadata[]> {
    const encodedPath = encodeURIComponent(path);
    return this.request<FileMetadata[]>(
      `/api/v1/sync/shares/${shareId}/list?path=${encodedPath}`
    );
  }

  /**
   * Get changes since cursor
   */
  async getChanges(shareId: string, cursor?: string): Promise<ChangesResponse> {
    const params = new URLSearchParams();
    if (cursor) {
      params.set('cursor', cursor);
    }
    const query = params.toString();
    return this.request<ChangesResponse>(
      `/api/v1/sync/shares/${shareId}/changes${query ? `?${query}` : ''}`
    );
  }

  /**
   * Get block hashes for delta sync
   */
  async getBlockHashes(shareId: string, request: BlockHashRequest): Promise<BlockHashResponse> {
    return this.request<BlockHashResponse>(
      `/api/v1/sync/shares/${shareId}/hash`,
      {
        method: 'POST',
        body: JSON.stringify(request),
      }
    );
  }

  // ==================== Sync State ====================

  /**
   * Get sync state for a share
   */
  async getSyncState(shareId: string): Promise<SyncState> {
    return this.request<SyncState>(`/api/v1/sync/state/${shareId}`);
  }

  /**
   * Update sync state
   */
  async updateSyncState(shareId: string, state: Partial<SyncState>): Promise<SyncState> {
    return this.request<SyncState>(`/api/v1/sync/state/${shareId}`, {
      method: 'PUT',
      body: JSON.stringify(state),
    });
  }

  // ==================== Conflicts ====================

  /**
   * List sync conflicts
   */
  async listConflicts(shareId?: string): Promise<SyncConflict[]> {
    const query = shareId ? `?share_id=${shareId}` : '';
    return this.request<SyncConflict[]>(`/api/v1/sync/conflicts${query}`);
  }

  /**
   * Resolve a conflict
   */
  async resolveConflict(
    conflictId: string,
    resolution: 'keep_local' | 'keep_remote' | 'keep_both'
  ): Promise<SyncConflict> {
    return this.request<SyncConflict>(`/api/v1/sync/conflicts/${conflictId}`, {
      method: 'PUT',
      body: JSON.stringify({resolution}),
    });
  }

  // ==================== Activity History ====================

  /**
   * Get sync activity history
   */
  async getActivityHistory(
    page: number = 1,
    pageSize: number = 50,
    shareId?: string
  ): Promise<ActivityListResponse> {
    const params = new URLSearchParams({
      page: page.toString(),
      page_size: pageSize.toString(),
    });
    if (shareId) {
      params.set('share_id', shareId);
    }
    return this.request<ActivityListResponse>(`/api/v1/sync/activity?${params}`);
  }

  /**
   * Get recent activities
   */
  async getRecentActivity(limit: number = 20): Promise<SyncActivity[]> {
    return this.request<SyncActivity[]>(`/api/v1/sync/activity/recent?limit=${limit}`);
  }

  // ==================== WebDAV URLs ====================

  /**
   * Get WebDAV URL for a share
   */
  getWebDAVUrl(shareId: string): string {
    return `${this.baseUrl}/dav/${shareId}`;
  }

  /**
   * Get WebDAV headers for authentication
   */
  getWebDAVHeaders(): Record<string, string> {
    return {
      Authorization: `Bearer ${this.deviceToken}`,
    };
  }

  // ==================== Helper Methods ====================

  /**
   * Make an authenticated API request
   */
  private async request<T>(
    endpoint: string,
    options: RequestInit = {},
    requireAuth: boolean = true
  ): Promise<T> {
    // Check token expiry and refresh if needed
    if (requireAuth && this.tokenExpires) {
      const expiresIn = this.tokenExpires.getTime() - Date.now();
      if (expiresIn < 5 * 60 * 1000) {
        // Less than 5 minutes
        await this.refreshAccessToken();
      }
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    };

    if (requireAuth && this.deviceToken) {
      headers.Authorization = `Bearer ${this.deviceToken}`;
    }

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      if (response.status === 401 && requireAuth) {
        // Token might be expired, try refresh
        await this.refreshAccessToken();
        // Retry request
        headers.Authorization = `Bearer ${this.deviceToken}`;
        const retryResponse = await fetch(`${this.baseUrl}${endpoint}`, {
          ...options,
          headers,
        });
        if (!retryResponse.ok) {
          throw await this.parseError(retryResponse);
        }
        return retryResponse.json();
      }
      throw await this.parseError(response);
    }

    // Handle empty responses
    const text = await response.text();
    if (!text) {
      return undefined as T;
    }
    return JSON.parse(text);
  }

  /**
   * Parse error response
   */
  private async parseError(response: Response): Promise<Error> {
    try {
      const data: APIError = await response.json();
      return new Error(data.message || `API Error: ${response.status}`);
    } catch {
      return new Error(`API Error: ${response.status} ${response.statusText}`);
    }
  }

  /**
   * Detect current platform
   */
  private detectPlatform(): 'ios' | 'android' {
    // React Native Platform module would be used here
    // For now, use a simple heuristic
    const userAgent = typeof navigator !== 'undefined' ? navigator.userAgent : '';
    if (userAgent.includes('iPhone') || userAgent.includes('iPad')) {
      return 'ios';
    }
    return 'android';
  }

  /**
   * Get OS version string
   */
  private getOSVersion(): string {
    // This would use React Native's Platform.Version
    return 'unknown';
  }
}

// Export singleton instance
export const apiClient = new NithronAPIClient();
export default apiClient;

