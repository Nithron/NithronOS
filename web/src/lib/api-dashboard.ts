// Dashboard API types and endpoints
import http from './nos-client'

// Dashboard types
export interface DashboardData {
  system: SystemSummary
  storage: StorageSummary
  disks: DisksSummary
  shares: ShareInfo[]
  apps: AppInfo[]
  maintenance: MaintenanceStatus
  events: EventInfo[]
}

export interface SystemSummary {
  status: 'ok' | 'degraded' | 'critical'
  cpuPct: number
  load1: number
  mem: {
    used: number
    total: number
  }
  swap: {
    used: number
    total: number
  }
  uptimeSec: number
}

export interface StorageSummary {
  totalBytes: number
  usedBytes: number
  poolsOnline: number
  poolsTotal: number
}

export interface DisksSummary {
  total: number
  healthy: number
  warning: number
  critical: number
  /** SMART had no answer for these disks. */
  unknown?: number
  /** When SMART was last read; absent before the first check. */
  lastScanISO?: string
}

export interface ShareInfo {
  name: string
  proto: 'SMB' | 'NFS' | 'AFP'
  path: string
  state: 'active' | 'disabled'
}

export interface AppInfo {
  id: string
  name: string
  state: string
  version: string
}

export interface MaintenanceStatus {
  scrub: MaintenanceOp
  balance: MaintenanceOp
}

export interface MaintenanceOp {
  state: 'idle' | 'running' | 'scheduled'
  nextISO: string
}

export interface EventInfo {
  id: string
  timestamp: string
  type: string
  message: string
  severity: 'info' | 'warning' | 'error'
}

// Dashboard API endpoints
export const dashboardApi = {
  // Get all dashboard data
  getDashboard: async (): Promise<DashboardData> => {
    // Let errors bubble up - the UI will handle them
    return await http.get<DashboardData>('/v1/dashboard')
  },

  // Individual endpoints for granular updates
  getSystemSummary: async (): Promise<SystemSummary> => {
    return await http.get<SystemSummary>('/v1/health/system')
  },

  getStorageSummary: async (): Promise<StorageSummary> => {
    return await http.get<StorageSummary>('/v1/storage/summary')
  },

  getDisksSummary: async (): Promise<DisksSummary> => {
    return await http.get<DisksSummary>('/v1/health/disks/summary')
  },

  // One row per protocol a share is exported over.
  getShares: async (): Promise<ShareInfo[]> => {
    type ApiShare = { name: string; path: string; smb?: { enabled: boolean }; nfs?: { enabled: boolean } }
    const response = await http.get<ApiShare[]>('/v1/shares')
    if (!Array.isArray(response)) return []
    return response.flatMap((s): ShareInfo[] => {
      const rows: ShareInfo[] = []
      if (s.smb) rows.push({ name: s.name, path: s.path, proto: 'SMB', state: s.smb.enabled ? 'active' : 'disabled' })
      if (s.nfs) rows.push({ name: s.name, path: s.path, proto: 'NFS', state: s.nfs.enabled ? 'active' : 'disabled' })
      return rows
    })
  },

  getInstalledApps: async (): Promise<AppInfo[]> => {
    const response = await http.get<AppInfo[]>('/v1/apps/installed')
    return Array.isArray(response) ? response : []
  },

  getMaintenanceStatus: async (): Promise<MaintenanceStatus> => {
    return await http.get<MaintenanceStatus>('/v1/maintenance/status')
  },

  getRecentEvents: async (limit = 10): Promise<EventInfo[]> => {
    const response = await http.get<EventInfo[]>(`/v1/events/recent?limit=${limit}`)
    return Array.isArray(response) ? response : []
  }
}
