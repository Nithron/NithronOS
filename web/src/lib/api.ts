// Shim for old API - redirects to nos-client
export * from './nos-client';
export { default } from './nos-client';

import http from './nos-client';

// Compatibility layer for old endpoints structure
export const endpoints = {
  system: {
    info: () => http.get('/v1/system/info'),
    metrics: () => http.get('/v1/system/metrics'),
    services: () => http.get('/v1/system/services'),
  },
  pools: {
    list: () => http.get('/v1/pools'),
    summary: () => http.get('/v1/pools/summary'),
    get: (uuid: string) => http.get(`/v1/pools/${uuid}`),
    subvolumes: (uuid: string) => http.get(`/v1/pools/${uuid}/subvolumes`),
    getMountOptions: (uuid: string) => http.get(`/v1/pools/${uuid}/mount-options`),
    setMountOptions: (uuid: string, options: any) => http.post(`/v1/pools/${uuid}/mount-options`, options),
  },
  devices: {
    list: () => http.get('/v1/devices'),
  },
  smart: {
    summary: () => http.get('/v1/smart/summary'),
    device: (device: string) => http.get(`/v1/smart/device/${device}`),
    scan: () => http.post('/v1/smart/scan'),
    devices: () => http.get('/v1/smart/devices'),
    test: (device: string) => http.get(`/v1/smart/test/${device}`),
    runTest: (device: string, type: string) => http.post(`/v1/smart/test/${device}`, { type }),
  },
  scrub: {
    status: () => http.get('/v1/scrub/status'),
    start: () => http.post('/v1/scrub/start'),
    cancel: () => http.post('/v1/scrub/cancel'),
  },
  balance: {
    status: () => http.get('/v1/balance/status'),
    start: () => http.post('/v1/balance/start'),
    cancel: () => http.post('/v1/balance/cancel'),
  },
  schedules: {
    list: () => http.get('/v1/schedules'),
    create: (schedule: any) => http.post('/v1/schedules', schedule),
    update: (id: string, schedule: any) => http.put(`/v1/schedules/${id}`, schedule),
    delete: (id: string) => http.del(`/v1/schedules/${id}`),
  },
  jobs: {
    recent: (limit: number) => http.get(`/v1/jobs/recent?limit=${limit}`),
  },
  shares: {
    list: () => http.get('/v1/shares'),
    get: (name: string) => http.get(`/v1/shares/${name}`),
    create: (share: any) => http.post('/v1/shares', share),
    update: (name: string, share: any) => http.put(`/v1/shares/${name}`, share),
    delete: (name: string) => http.del(`/v1/shares/${name}`),
    test: (name: string, config: any) => http.post(`/v1/shares/${name}/test`, config),
  },
  apps: {
    catalog: () => http.get('/v1/apps/catalog'),
    installed: () => http.get('/v1/apps/installed'),
    get: (id: string) => http.get(`/v1/apps/${id}`),
    install: (app: any) => http.post('/v1/apps/install', app),
    start: (id: string) => http.post(`/v1/apps/${id}/start`),
    stop: (id: string) => http.post(`/v1/apps/${id}/stop`),
    restart: (id: string) => http.post(`/v1/apps/${id}/restart`),
    upgrade: (id: string, params: any) => http.post(`/v1/apps/${id}/upgrade`, params),
    delete: (id: string, keepData: boolean) => http.del(`/v1/apps/${id}?keep_data=${keepData}`),
  },
  monitoring: {
    getLogs: () => http.get('/v1/monitoring/logs') || (() => Promise.resolve([])),
    getEvents: () => http.get('/v1/monitoring/events') || (() => Promise.resolve([])),
    getAlerts: () => http.get('/v1/monitoring/alerts') || (() => Promise.resolve([])),
    getServiceStatus: () => http.get('/v1/monitoring/services') || (() => Promise.resolve([])),
  },
};

// Re-export common types
export interface ScrubStatus {
  running: boolean;
  progress?: number;
  eta?: string;
}

export interface BalanceStatus {
  running: boolean;
  progress?: number;
  eta?: string;
}

export interface Schedule {
  id: string;
  name: string;
  cron: string;
  enabled: boolean;
}

export interface Share {
  name: string;
  path: string;
  protocol: string;
  enabled: boolean;
}