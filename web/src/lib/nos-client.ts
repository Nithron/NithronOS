import axios, { AxiosError, AxiosInstance } from 'axios';

export type ApiError = {
  code: string;              // e.g. "Unauthorized", "RateLimited"
  message: string;           // human-friendly
  details?: unknown;         // optional structured data
  status?: number;           // HTTP status
  requestId?: string;        // from server logs if present
};

// Backward compatibility type exports
export interface AuthSession {
  user: {
    id: string;
    username: string;
    isAdmin?: boolean;
    roles?: string[];
  };
  expiresAt?: string;
}

// Backward compatibility aliases
export class APIError extends Error {
  status?: number;
  code?: string;
  retryAfterSec?: number;
  
  constructor(message: string, status?: number) {
    super(message);
    this.status = status;
  }
}

export class ProxyMisconfiguredError extends Error {
  constructor(message: string) {
    super(message);
  }
}

export function getErrorMessage(err: any): string {
  if (err?.message) return err.message;
  if (typeof err === 'string') return err;
  return 'An error occurred';
}

function readCookie(name: string): string | null {
  const m = document.cookie.match(new RegExp('(^|; )' + name.replace(/([.*+?^${}()|[\]\\])/g, '\\$1') + '=([^;]*)'));
  return m ? decodeURIComponent(m[2]) : null;
}

let refreshing: Promise<void> | null = null;

// Relaxed path validation to support various API patterns
const ALLOWED_PATH_PREFIXES = [
  '/v1/',
  '/v1/setup/',
  '/auth/',
  '/metrics',
  '/debug/pprof',
  '/events',
  '/ws',
  '/pools',
  '/disks',
  '/shares',
  '/apps',
  '/snapshots',
  '/support',
  '/updates',
  '/firewall',
  '/smb',
  '/health',
  '/storage',
  '/sync',
  '/dav',
];

function assertValidPath(path: string) {
  if (!path.startsWith('/')) {
    throw new Error("Path must start with '/'");
  }
  
  // Dev-only guard: prevent including /api in paths
  if (process.env.NODE_ENV === 'development' && path.startsWith('/api/')) {
    throw new Error("Do not include '/api' in paths; baseURL is already '/api'.");
  }
  
  // In production, be more lenient but still validate
  const isValid = ALLOWED_PATH_PREFIXES.some(prefix => path.startsWith(prefix));
  if (!isValid && process.env.NODE_ENV === 'development') {
    console.warn(`Unusual API path: ${path}. Consider adding to ALLOWED_PATH_PREFIXES if intentional.`);
  }
}

function toApiError(err: any): ApiError {
  if (err?.isAxiosError) {
    const ae = err as AxiosError<any>;
    const status = ae.response?.status;
    const data = ae.response?.data;
    // Try to normalize server error shape; fall back sensibly
    const code = (data?.code as string) || (status === 401 ? 'Unauthorized' : status === 429 ? 'RateLimited' : 'HttpError');
    const message = (typeof data?.message === 'string' ? data.message : ae.message) || 'Request failed';
    const requestId = ae.response?.headers?.['x-request-id'];
    return { code, message, details: data?.details ?? data, status, requestId };
  }
  return { code: 'UnknownError', message: err?.message || 'Unknown error' };
}

const client: AxiosInstance = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 15000,
  headers: { 'Accept': 'application/json' }
});

// Request interceptor: add CSRF header if cookie exists
client.interceptors.request.use((config) => {
  const csrf = readCookie('nos_csrf') || readCookie('csrf_token');
  if (csrf && config.headers) {
    config.headers['X-CSRF-Token'] = csrf;
  }
  // Guard path correctness for relative URLs
  const url = (config.url || '');
  if (url.startsWith('/')) assertValidPath(url);
  return config;
});

// Response interceptor: on 401, try single refresh then replay once
client.interceptors.response.use(
  (res) => res,
  async (error) => {
    const err = toApiError(error);
    const original = error?.config;
    if (err.status === 401 && !original?._retry) {
      if (!refreshing) {
        refreshing = (async () => {
          try { await client.post('/v1/auth/refresh', {}); }
          finally { refreshing = null; }
        })();
      }
      await refreshing;
      original._retry = true;
      return client(original);
    }
    // For backward compatibility, throw APIError instances
    const apiErr = new APIError(err.message, err.status);
    apiErr.code = err.code;
    if ((err as any).retryAfterSec) apiErr.retryAfterSec = (err as any).retryAfterSec;
    return Promise.reject(apiErr);
  }
);

// Minimal typed wrapper
const httpCore = {
  async get<T>(path: string, params?: Record<string, any>): Promise<T> {
    assertValidPath(path);
    const res = await client.get<T>(path, { params });
    return res.data;
  },
  async post<T>(path: string, body?: any): Promise<T> {
    assertValidPath(path);
    const res = await client.post<T>(path, body ?? {});
    return res.data;
  },
  async put<T>(path: string, body?: any): Promise<T> {
    assertValidPath(path);
    const res = await client.put<T>(path, body ?? {});
    return res.data;
  },
  async patch<T>(path: string, body?: any): Promise<T> {
    assertValidPath(path);
    const res = await client.patch<T>(path, body ?? {});
    return res.data;
  },
  async del<T>(path: string): Promise<T> {
    assertValidPath(path);
    const res = await client.delete<T>(path);
    return res.data;
  },
  
  // Special method for binary downloads
  async getBlob(path: string, params?: Record<string, any>): Promise<Blob> {
    assertValidPath(path);
    const res = await client.get(path, { params, responseType: 'blob' });
    return res.data;
  }
};

// SSE (Server-Sent Events) helper
export function openSSE(path: string, options?: EventSourceInit): EventSource {
  assertValidPath(path);
  const url = `${window.location.origin}/api${path}`;
  return new EventSource(url, { 
    withCredentials: true,
    ...options 
  });
}

// WebSocket helper
export function openWS(path: string): WebSocket {
  assertValidPath(path);
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const url = `${protocol}//${host}/api${path}`;
  return new WebSocket(url);
}

// Extended API client with nested structure for backward compatibility
export const http = {
  ...httpCore,
  delete: httpCore.del,  // Add delete method alias
  
  // Setup endpoints
  setup: {
    getState: () => httpCore.get<any>('/v1/setup/state'),
    verifyOTP: (otp: string) => httpCore.post('/v1/setup/otp/verify', { otp }),
    createAdmin: (token: string, data: any) => httpCore.post('/v1/setup/first-admin', { token, ...data }),
    complete: (token: string) => httpCore.post('/v1/setup/complete', { token }),
  },
  
  // Auth endpoints
  auth: {
    login: (data: any) => httpCore.post('/v1/auth/login', data),
    logout: () => httpCore.post('/v1/auth/logout'),
    refresh: () => httpCore.post('/v1/auth/refresh', {}),
    me: () => httpCore.get('/v1/auth/me'),
    verifyTotp: (code: string) => httpCore.post('/v1/auth/verify-totp', { code }),
    getSession: () => httpCore.get('/v1/auth/session'),
    session: () => httpCore.get('/v1/auth/session'),
    totp: {
      enroll: () => httpCore.post('/v1/auth/totp/enroll'),
      verify: (code: string) => httpCore.post('/v1/auth/totp/verify', { code }),
    },
  },
  
  // System endpoints
  system: {
    info: () => httpCore.get('/v1/system/info'),
    metrics: () => httpCore.get('/v1/system/metrics'),
    services: () => httpCore.get('/v1/system/services'),
    getTimezone: () => httpCore.get('/v1/system/timezone'),
    getNTP: () => httpCore.get('/v1/system/ntp'),
    setHostname: (data: any) => httpCore.post('/v1/system/hostname', data),
    setTimezone: (data: any) => httpCore.post('/v1/system/timezone', data),
    setNTP: (data: any) => httpCore.post('/v1/system/ntp', data),
  },
  
  // Network endpoints
  network: {
    getInterfaces: () => httpCore.get('/v1/network/interfaces'),
    configureInterface: (iface: string, config: any) => httpCore.post(`/v1/network/interfaces/${iface}`, config),
  },
  
  // Telemetry endpoints
  telemetry: {
    setConsent: (data: any) => httpCore.post('/v1/telemetry/consent', data),
  },
  
  // Health endpoints
  health: {
    system: () => httpCore.get('/v1/health/system'),
    disks: () => httpCore.get('/v1/health/disks'),
    disksSummary: () => httpCore.get('/v1/health/disks/summary'),
    smart: (device?: string) => device 
      ? httpCore.get(`/v1/health/smart/${device}`)
      : httpCore.get('/v1/health/smart'),
  },
  
  // Storage endpoints
  storage: {
    summary: () => httpCore.get('/v1/storage/summary'),
    pools: () => httpCore.get('/v1/storage/pools'),
  },
  
  // Pool endpoints
  pools: {
    get: (id: string) => httpCore.get(`/v1/pools/${id}`),
    list: () => httpCore.get('/v1/pools'),
    create: (data: any) => httpCore.post('/v1/pools', data),
    delete: (id: string) => httpCore.del(`/v1/pools/${id}`),
    getMountOptions: (id: string) => httpCore.get(`/v1/pools/${id}/mount-options`),
    updateMountOptions: (id: string, options: any) => httpCore.post(`/v1/pools/${id}/mount-options`, options),
    planDevice: (id: string, body: any) => httpCore.post(`/v1/pools/${id}/plan-device`, body),
    applyDevice: (id: string, body: any) => httpCore.post(`/v1/pools/${id}/apply-device`, body),
    roots: () => httpCore.get('/v1/pools/roots'),
    snapshots: (id: string) => httpCore.get(`/v1/pools/${id}/snapshots`),
    // Legacy unversioned endpoints (if still in use)
    listUnversioned: () => httpCore.get('/pools'),
    snapshotsUnversioned: (id: string) => httpCore.get(`/pools/${id}/snapshots`),
    // Transaction endpoints
    txStatus: (id: string) => httpCore.get(`/v1/pools/tx/${id}/status`),
    txLog: (id: string, cursor = 0, max = 1000) => 
      httpCore.get(`/v1/pools/tx/${id}/log`, { cursor, max }),
    txStream: (id: string) => openSSE(`/v1/pools/tx/${id}/stream`),
  },
  
  // Disk endpoints
  disks: {
    list: () => httpCore.get('/disks'),
    listV1: () => httpCore.get('/v1/disks'),
  },
  
  // Snapshot endpoints
  snapshots: {
    recent: () => httpCore.get('/snapshots/recent'),
    prune: (data: any) => httpCore.post('/snapshots/prune', data),
  },
  
  // Updates endpoints
  updates: {
    check: () => httpCore.get('/v1/updates/check'),
    apply: (data: any) => httpCore.post('/v1/updates/apply', data),
    rollback: (data: any) => httpCore.post('/v1/updates/rollback', data),
    streamProgress: () => openSSE('/v1/updates/progress/stream'),
  },
  
  // Shares endpoints
  shares: {
    list: () => httpCore.get('/v1/shares'),
    get: (name: string) => httpCore.get(`/v1/shares/${name}`),
    create: (data: any) => httpCore.post('/v1/shares', data),
    update: (name: string, data: any) => httpCore.put(`/v1/shares/${name}`, data),
    delete: (name: string) => httpCore.del(`/v1/shares/${name}`),
    test: (name: string, config: any) => httpCore.post(`/v1/shares/${name}/test`, config),
  },
  
  // SMB endpoints
  smb: {
    usersList: () => httpCore.get('/v1/smb/users'),
    userCreate: (data: any) => httpCore.post('/v1/smb/users', data),
  },
  
  // Apps endpoints
  apps: {
    catalog: () => httpCore.get('/v1/apps/catalog'),
    installed: () => httpCore.get('/v1/apps/installed'),
    get: (id: string) => httpCore.get(`/v1/apps/${id}`),
    install: (data: any) => httpCore.post('/v1/apps/install', data),
    start: (id: string) => httpCore.post(`/v1/apps/${id}/start`),
    stop: (id: string) => httpCore.post(`/v1/apps/${id}/stop`),
    restart: (id: string) => httpCore.post(`/v1/apps/${id}/restart`),
    upgrade: (id: string, params: any) => httpCore.post(`/v1/apps/${id}/upgrade`, params),
    delete: (id: string, keepData: boolean) => httpCore.del(`/v1/apps/${id}?keep_data=${keepData}`),
    schema: (id: string) => httpCore.get(`/v1/apps/schema/${id}`),
    streamLogs: (id: string, options: any = {}) => {
      const params = new URLSearchParams();
      if (options.follow) params.append('follow', '1');
      if (options.tail) params.append('tail', options.tail.toString());
      if (options.timestamps) params.append('timestamps', 'true');
      if (options.container) params.append('container', options.container);
      return openWS(`/v1/apps/${id}/logs?${params}`);
    },
  },
  
  // Support endpoints
  support: {
    bundle: () => httpCore.getBlob('/support/bundle'),
  },
  
  // Monitoring endpoints
  monitoring: {
    system: () => httpCore.get('/v1/monitoring/system'),
    logs: () => httpCore.get('/v1/monitoring/logs'),
    events: () => httpCore.get('/v1/monitoring/events'),
    alerts: () => httpCore.get('/v1/monitoring/alerts'),
    services: () => httpCore.get('/v1/monitoring/services'),
  },
  
  // Additional helper groups for specific features
  scrub: {
    status: () => httpCore.get('/v1/scrub/status'),
    start: () => httpCore.post('/v1/scrub/start'),
    cancel: () => httpCore.post('/v1/scrub/cancel'),
  },
  
  balance: {
    status: () => httpCore.get('/v1/balance/status'),
    start: () => httpCore.post('/v1/balance/start'),
    cancel: () => httpCore.post('/v1/balance/cancel'),
  },
  
  schedules: {
    list: () => httpCore.get('/v1/schedules'),
    create: (schedule: any) => httpCore.post('/v1/schedules', schedule),
    update: (id: string, schedule: any) => httpCore.put(`/v1/schedules/${id}`, schedule),
    delete: (id: string) => httpCore.del(`/v1/schedules/${id}`),
  },
  
  jobs: {
    recent: (limit: number) => httpCore.get(`/v1/jobs/recent?limit=${limit}`),
  },
  
  devices: {
    list: () => httpCore.get('/v1/devices'),
  },
  
  smart: {
    summary: () => httpCore.get('/v1/smart/summary'),
    device: (device: string) => httpCore.get(`/v1/smart/device/${device}`),
    scan: () => httpCore.post('/v1/smart/scan'),
    devices: () => httpCore.get('/v1/smart/devices'),
    test: (device: string) => httpCore.get(`/v1/smart/test/${device}`),
    runTest: (device: string, type: string) => httpCore.post(`/v1/smart/test/${device}`, { type }),
  },
};

// Explicit FE contract of endpoints used by the app (method + path)
export const NOS_ENDPOINTS: Array<{ method: 'GET'|'POST'|'PUT'|'PATCH'|'DELETE', path: string }> = [
  // Setup
  { method: 'GET', path: '/api/v1/setup/state' },
  { method: 'POST', path: '/api/v1/setup/otp/verify' },
  { method: 'POST', path: '/api/v1/setup/first-admin' },
  { method: 'POST', path: '/api/v1/setup/complete' },
  // Auth
  { method: 'POST', path: '/api/v1/auth/login' },
  { method: 'POST', path: '/api/v1/auth/logout' },
  { method: 'POST', path: '/api/v1/auth/refresh' },
  { method: 'GET', path: '/api/v1/auth/me' },
  { method: 'GET', path: '/api/v1/auth/session' },
  { method: 'GET', path: '/api/v1/auth/sessions' },
  { method: 'POST', path: '/api/v1/auth/sessions/revoke' },
  { method: 'POST', path: '/api/v1/auth/totp/verify' },
  { method: 'POST', path: '/api/v1/auth/totp/enroll' },
  // System/health
  { method: 'GET', path: '/api/v1/system/info' },
  { method: 'GET', path: '/api/v1/health/system' },
  { method: 'GET', path: '/api/v1/health/disks' },
  { method: 'GET', path: '/api/v1/health/smart' },
  // Storage/pools (subset used)
  { method: 'GET', path: '/api/v1/storage/summary' },
  { method: 'GET', path: '/api/v1/pools' },
  { method: 'GET', path: '/api/v1/pools/{id}' },
  { method: 'POST', path: '/api/v1/pools/{id}/plan-device' },
  { method: 'POST', path: '/api/v1/pools/{id}/apply-device' },
  { method: 'GET', path: '/api/v1/pools/{id}/mount-options' },
  { method: 'POST', path: '/api/v1/pools/{id}/mount-options' },
  { method: 'GET', path: '/api/v1/pools/tx/{id}/log' },
  // Shares
  { method: 'GET', path: '/api/v1/shares' },
  { method: 'GET', path: '/api/v1/shares/{name}' },
  { method: 'POST', path: '/api/v1/shares' },
  { method: 'PUT', path: '/api/v1/shares/{name}' },
  { method: 'DELETE', path: '/api/v1/shares/{name}' },
  // Updates
  { method: 'GET', path: '/api/v1/updates/check' },
  { method: 'POST', path: '/api/v1/updates/apply' },
  { method: 'POST', path: '/api/v1/updates/rollback' },
  // Apps
  { method: 'GET', path: '/api/v1/apps/installed' },
  { method: 'GET', path: '/api/v1/apps/catalog' },
  { method: 'GET', path: '/api/v1/apps/{id}' },
];

// Backward compatibility - export api as an alias to http
export const api = http;

// Export nos as an alias for cleaner imports
export const nos = http;

// Re-export the base http methods as default
export default http;