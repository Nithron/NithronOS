import http from '@/lib/nos-client';
import type {
  AppSnapshot,
  Catalog,
  CatalogEntry,
  InstalledApp,
  InstallRequest,
  UpgradeRequest,
  RollbackRequest,
  LogStreamOptions,
  AppEvent,
} from './apps.types';

/** An app's install form: a JSON Schema object (the template's schema.json). */
export interface AppSchema {
  type?: string;
  title?: string;
  properties?: Record<string, any>;
  required?: string[];
}

export const appsApi = {
  // Catalog operations
  getCatalog: () =>
    http.get<Catalog>('/v1/apps/catalog'),

  // The install form (JSON Schema) of a catalog app.
  getSchema: (id: string) =>
    http.get<AppSchema>(`/v1/apps/catalog/${encodeURIComponent(id)}/schema`),

  getInstalledApps: () =>
    http.get<{ items: InstalledApp[] }>('/v1/apps/installed'),

  getApp: (id: string) =>
    http.get<InstalledApp>(`/v1/apps/${id}`),

  // App lifecycle operations
  installApp: (data: InstallRequest) =>
    http.post<{ message: string; app: InstalledApp }>('/v1/apps/install', data),

  upgradeApp: (id: string, data: UpgradeRequest) =>
    http.post<{ message: string; version: string }>(`/v1/apps/${id}/upgrade`, data),

  // A snapshot of the app's data, now.
  snapshotApp: (id: string) =>
    http.post<AppSnapshot>(`/v1/apps/${id}/snapshots`),

  startApp: (id: string) =>
    http.post<{ message: string }>(`/v1/apps/${id}/start`),

  stopApp: (id: string) =>
    http.post<{ message: string }>(`/v1/apps/${id}/stop`),

  restartApp: (id: string) =>
    http.post<{ message: string }>(`/v1/apps/${id}/restart`),

  deleteApp: (id: string, keepData = false) =>
    http.del<{ message: string }>(`/v1/apps/${id}?keep_data=${keepData}`),

  rollbackApp: (id: string, snapshotTs: string) =>
    http.post<{ message: string }>(`/v1/apps/${id}/rollback`, {
      snapshot_ts: snapshotTs,
    } as RollbackRequest),

  // Monitoring operations
  forceHealthCheck: (id: string) =>
    http.post<{ message: string; health: any }>(`/v1/apps/${id}/health`),

  getAppEvents: (id: string, limit = 100) =>
    http.get<{ events: AppEvent[] }>(`/v1/apps/${id}/events?limit=${limit}`),

  // Admin operations
  syncCatalogs: () =>
    http.post<{ message: string }>('/v1/apps/catalog/sync'),

  // WebSocket for logs
  streamLogs: (id: string, options: LogStreamOptions = {}) => {
    return http.apps.streamLogs(id, options);
  },

  // Get logs (non-streaming)
  getLogs: (id: string, options: LogStreamOptions = {}) => {
    const params = new URLSearchParams();
    if (options.tail) params.append('tail', options.tail.toString());
    if (options.timestamps) params.append('timestamps', 'true');
    if (options.container) params.append('container', options.container);

    return http.get<string>(`/v1/apps/${id}/logs?${params}`);
  },
};

// Helper to load schema from catalog entry
export async function loadAppSchema(catalogEntry: CatalogEntry): Promise<any> {
  if (!catalogEntry.schema) return null;
  
  // In production, schemas would be loaded from the server
  // For now, we'll parse them from the catalog
  try {
    return await http.apps.schema(catalogEntry.id);
  } catch (error) {
    console.error('Failed to load schema:', error);
  }
  
  return null;
}
