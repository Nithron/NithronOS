/**
 * The app's connection to nosd: the sync API (/api/v1/sync, docs/sync/api.md)
 * and WebDAV (/dav), with the device token from the pairing link.
 *
 * A request that gets 401 refreshes the tokens (both rotate) and is sent
 * again once; when the refresh token is refused too, the pairing is over and
 * onSessionEnded fires.
 */

import {
  clearCredentials,
  Credentials,
  loadCredentials,
  saveCredentials,
} from './credentials';
import {Pairing, serverHost} from './pairing';
import {
  ActivityListResponse,
  ConflictResolution,
  Device,
  ErrorBody,
  FileMetadata,
  FolderEntry,
  RefreshResponse,
  ShareListResponse,
  SyncActivity,
  SyncConfig,
  SyncConflict,
  SyncShare,
} from './types';
import {davPath, parseFolderListing} from './webdav';
import {clearServerPins, setServerPin} from '../native/pinning';

const REQUEST_TIMEOUT_MS = 30_000;
const HTTP_UNAUTHORIZED = 401;

export class ApiError extends Error {
  readonly status: number;
  /** nosd's error code (e.g. share.not_found), when it gave one */
  readonly code: string | null;

  constructor(status: number, message: string, code: string | null = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

/** The pairing can't be used any more: the server refused its tokens. */
export class SessionEndedError extends ApiError {
  constructor() {
    super(
      HTTP_UNAUTHORIZED,
      'This device was signed out by the server. Pair it again.',
      'auth.invalid',
    );
    this.name = 'SessionEndedError';
  }
}

type FetchFn = typeof fetch;

interface RequestOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
}

export class NosClient {
  private credentials: Credentials | null = null;
  private refreshing: Promise<void> | null = null;
  private readonly fetchFn: FetchFn;

  /** Fires when the server refuses the device's tokens for good. */
  onSessionEnded: (() => void) | null = null;

  constructor(fetchFn: FetchFn = (...args) => fetch(...args)) {
    this.fetchFn = fetchFn;
  }

  get isPaired(): boolean {
    return this.credentials !== null;
  }

  get serverUrl(): string | null {
    return this.credentials?.serverUrl ?? null;
  }

  /** Loads the saved pairing; false when there is none. */
  async restore(): Promise<boolean> {
    const saved = await loadCredentials();
    if (!saved) {
      return false;
    }
    await this.pinServer(saved);
    this.credentials = saved;
    return true;
  }

  /**
   * Pairs with the server in link: trusts its key, checks that the token
   * works, and only then keeps the pairing.
   */
  async pair(pairing: Pairing): Promise<SyncConfig> {
    const credentials: Credentials = {
      serverUrl: pairing.serverUrl,
      deviceToken: pairing.deviceToken,
      refreshToken: pairing.refreshToken,
      serverKey: pairing.serverKey,
    };
    // One server at a time: no pin of an earlier pairing stays trusted.
    await clearServerPins();
    await this.pinServer(credentials);
    this.credentials = credentials;
    try {
      const config = await this.getConfig();
      await saveCredentials(this.requireCredentials());
      return config;
    } catch (error) {
      this.credentials = null;
      await clearServerPins();
      throw error;
    }
  }

  /** Drops the pairing on this device (the server isn't told). */
  async forget(): Promise<void> {
    this.credentials = null;
    await clearCredentials();
    await clearServerPins();
  }

  // ==================== Sync API ====================

  /** This device's sync settings; device_id is this device's id. */
  getConfig(): Promise<SyncConfig> {
    return this.json<SyncConfig>('/api/v1/sync/config');
  }

  getDevice(deviceId: string): Promise<Device> {
    return this.json<Device>(
      `/api/v1/sync/devices/${encodeURIComponent(deviceId)}`,
    );
  }

  renameDevice(deviceId: string, name: string): Promise<Device> {
    return this.json<Device>(
      `/api/v1/sync/devices/${encodeURIComponent(deviceId)}`,
      {method: 'PATCH', body: {device_name: name}},
    );
  }

  async revokeDevice(deviceId: string): Promise<void> {
    await this.json<void>(
      `/api/v1/sync/devices/${encodeURIComponent(deviceId)}`,
      {method: 'DELETE'},
    );
  }

  async listShares(): Promise<SyncShare[]> {
    const body = await this.json<ShareListResponse>('/api/v1/sync/shares');
    return body.shares ?? [];
  }

  getFileMetadata(shareId: string, path: string): Promise<FileMetadata> {
    const query = `path=${encodeURIComponent(path)}`;
    return this.json<FileMetadata>(
      `/api/v1/sync/files/${encodeURIComponent(shareId)}/metadata?${query}`,
    );
  }

  async listConflicts(): Promise<SyncConflict[]> {
    return (
      (await this.json<SyncConflict[] | null>('/api/v1/sync/conflicts')) ?? []
    );
  }

  resolveConflict(
    conflictId: string,
    resolution: ConflictResolution,
  ): Promise<SyncConflict> {
    return this.json<SyncConflict>(
      `/api/v1/sync/conflicts/${encodeURIComponent(conflictId)}`,
      {method: 'PUT', body: {resolution}},
    );
  }

  async recentActivity(limit: number): Promise<SyncActivity[]> {
    const path = `/api/v1/sync/activity/recent?limit=${limit}`;
    return (await this.json<SyncActivity[] | null>(path)) ?? [];
  }

  async activityPage(
    page: number,
    pageSize: number,
  ): Promise<ActivityListResponse> {
    const path = `/api/v1/sync/activity?page=${page}&page_size=${pageSize}`;
    const body = await this.json<ActivityListResponse>(path);
    return {...body, activities: body.activities ?? []};
  }

  // ==================== WebDAV ====================

  async listFolder(shareId: string, path: string): Promise<FolderEntry[]> {
    const response = await this.send(davPath(shareId, path, true), {
      method: 'PROPFIND',
      headers: {Depth: '1'},
    });
    if (response.status !== 207 && !response.ok) {
      throw await davError(response);
    }
    return parseFolderListing(await response.text(), shareId, path);
  }

  async deletePath(
    shareId: string,
    path: string,
    isDir: boolean,
  ): Promise<void> {
    const response = await this.send(davPath(shareId, path, isDir), {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw await davError(response);
    }
  }

  // ==================== Requests ====================

  private async json<T>(
    path: string,
    options: RequestOptions = {},
  ): Promise<T> {
    const response = await this.send(path, {
      ...options,
      headers: {Accept: 'application/json', ...options.headers},
    });
    if (!response.ok) {
      throw await apiError(response);
    }
    const text = await response.text();
    return (text === '' ? undefined : JSON.parse(text)) as T;
  }

  /** Sends with the device token; after a 401, refreshes and sends once more. */
  private async send(path: string, options: RequestOptions): Promise<Response> {
    const sentWith = this.requireCredentials().deviceToken;
    const response = await this.fetchOnce(path, options, sentWith);
    if (response.status !== HTTP_UNAUTHORIZED) {
      return response;
    }
    await this.refreshAfter(sentWith);
    return this.fetchOnce(path, options, this.requireCredentials().deviceToken);
  }

  private async fetchOnce(
    path: string,
    options: RequestOptions,
    token: string,
  ): Promise<Response> {
    const {serverUrl} = this.requireCredentials();
    const headers: Record<string, string> = {
      Authorization: `Bearer ${token}`,
      ...options.headers,
    };
    let body: string | undefined;
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }
    return this.timedFetch(`${serverUrl}${path}`, {
      method: options.method ?? 'GET',
      headers,
      body,
    });
  }

  /**
   * Gets new tokens unless another request already did since token was
   * used. Concurrent callers share one refresh.
   */
  private async refreshAfter(token: string): Promise<void> {
    if (this.requireCredentials().deviceToken !== token) {
      return;
    }
    if (!this.refreshing) {
      this.refreshing = this.refreshTokens().finally(() => {
        this.refreshing = null;
      });
    }
    await this.refreshing;
  }

  private async refreshTokens(): Promise<void> {
    const current = this.requireCredentials();
    if (!current.refreshToken) {
      return this.endSession();
    }
    const response = await this.timedFetch(
      `${current.serverUrl}/api/v1/sync/devices/refresh`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
        body: JSON.stringify({refresh_token: current.refreshToken}),
      },
    );
    if (response.status === HTTP_UNAUTHORIZED) {
      return this.endSession();
    }
    if (!response.ok) {
      throw await apiError(response);
    }
    const tokens = (await response.json()) as RefreshResponse;
    const next: Credentials = {
      ...current,
      deviceToken: tokens.device_token,
      refreshToken: tokens.refresh_token,
    };
    await saveCredentials(next);
    this.credentials = next;
  }

  private async endSession(): Promise<never> {
    await this.forget();
    this.onSessionEnded?.();
    throw new SessionEndedError();
  }

  private async timedFetch(url: string, init: RequestInit): Promise<Response> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    try {
      return await this.fetchFn(url, {...init, signal: controller.signal});
    } catch (error) {
      const pinned = Boolean(this.credentials?.serverKey);
      throw new Error(connectionMessage(url, error, pinned));
    } finally {
      clearTimeout(timer);
    }
  }

  private async pinServer(credentials: Credentials): Promise<void> {
    const host = serverHost(credentials.serverUrl);
    if (credentials.serverKey && host) {
      await setServerPin(host, credentials.serverKey);
    }
  }

  private requireCredentials(): Credentials {
    if (!this.credentials) {
      throw new ApiError(
        HTTP_UNAUTHORIZED,
        'This device is not paired',
        'auth.required',
      );
    }
    return this.credentials;
  }
}

async function apiError(response: Response): Promise<ApiError> {
  let body: ErrorBody = {};
  try {
    body = (await response.json()) as ErrorBody;
  } catch {
    // Not nosd's JSON (a proxy's page, an empty body): the status says it.
  }
  const message =
    body.error?.message || `The server answered ${response.status}`;
  return new ApiError(response.status, message, body.error?.code ?? null);
}

async function davError(response: Response): Promise<ApiError> {
  let text = '';
  try {
    text = (await response.text()).trim();
  } catch {
    // The status alone says enough.
  }
  return new ApiError(
    response.status,
    text || `The server answered ${response.status}`,
  );
}

function connectionMessage(
  url: string,
  error: unknown,
  pinned: boolean,
): string {
  const reason = error instanceof Error ? error.message : String(error);
  if (/abort/i.test(reason)) {
    return `${serverOrigin(url)} didn't answer in time`;
  }
  // React Native doesn't say why; a refused key looks the same as no server.
  const keyHint = pinned
    ? " If it's running, its TLS key may not be the one in the pairing link."
    : '';
  return `Couldn't reach ${serverOrigin(url)} (${reason}).${keyHint}`;
}

function serverOrigin(url: string): string {
  return /^[a-z]+:\/\/[^/]+/i.exec(url)?.[0] ?? url;
}

export const apiClient = new NosClient();
export default apiClient;
