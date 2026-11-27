// Dashboard hooks with 1Hz refresh
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { 
  dashboardApi, 
  type DashboardData,
  type SystemSummary,
  type StorageSummary,
  type DisksSummary,
  type ShareInfo,
  type AppInfo,
  type MaintenanceStatus,
  type EventInfo
} from '@/lib/api-dashboard'

// Main dashboard hook - aggregated data with auto-refresh
export function useDashboard() {
  return useQuery<DashboardData>({
    queryKey: ['dashboard'],
    queryFn: dashboardApi.getDashboard,
    refetchInterval: 5000, // Refresh every 5 seconds for dashboard (less aggressive than monitoring)
    refetchIntervalInBackground: true,
    staleTime: 2000, // Consider data stale after 2 seconds
    gcTime: 60000, // Keep in cache for 1 minute
    retry: 1,
    retryDelay: 1000,
  })
}

// Individual widget hooks for granular updates
export function useSystemSummary() {
  return useQuery<SystemSummary>({
    queryKey: ['dashboard', 'system'],
    queryFn: dashboardApi.getSystemSummary,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
  })
}

export function useStorageSummary() {
  return useQuery<StorageSummary>({
    queryKey: ['dashboard', 'storage'],
    queryFn: dashboardApi.getStorageSummary,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
  })
}

export function useDisksSummary() {
  return useQuery<DisksSummary>({
    queryKey: ['dashboard', 'disks'],
    queryFn: dashboardApi.getDisksSummary,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
  })
}

export function useShares() {
  return useQuery<ShareInfo[]>({
    queryKey: ['dashboard', 'shares'],
    queryFn: dashboardApi.getShares,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
    select: (data) => Array.isArray(data) ? data : [],
  })
}

export function useInstalledApps() {
  return useQuery<AppInfo[]>({
    queryKey: ['dashboard', 'apps'],
    queryFn: dashboardApi.getInstalledApps,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
    select: (data) => Array.isArray(data) ? data : [],
  })
}

export function useMaintenanceStatus() {
  return useQuery<MaintenanceStatus>({
    queryKey: ['dashboard', 'maintenance'],
    queryFn: dashboardApi.getMaintenanceStatus,
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
  })
}

export function useRecentEvents(limit = 10) {
  return useQuery<EventInfo[]>({
    queryKey: ['dashboard', 'events', limit],
    queryFn: () => dashboardApi.getRecentEvents(limit),
    refetchInterval: 1000,
    refetchIntervalInBackground: true,
    staleTime: 500,
    gcTime: 60000,
    retry: 1,
    select: (data) => Array.isArray(data) ? data : [],
  })
}

// Refresh all dashboard queries
export function useRefreshDashboard() {
  const queryClient = useQueryClient()
  
  return () => {
    // Invalidate all dashboard queries
    queryClient.invalidateQueries({ queryKey: ['dashboard'] })
  }
}

// Helper functions
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

export function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  
  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m`
  } else if (hours > 0) {
    return `${hours}h ${minutes}m`
  } else {
    return `${minutes}m`
  }
}

export function formatTimestamp(iso: string): string {
  const date = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  
  // Less than a minute
  if (diff < 60000) {
    return 'Just now'
  }
  
  // Less than an hour
  if (diff < 3600000) {
    const minutes = Math.floor(diff / 60000)
    return `${minutes}m ago`
  }
  
  // Less than a day
  if (diff < 86400000) {
    const hours = Math.floor(diff / 3600000)
    return `${hours}h ago`
  }
  
  // More than a day
  const days = Math.floor(diff / 86400000)
  return `${days}d ago`
}

export function getHealthColor(status: 'ok' | 'degraded' | 'critical'): string {
  switch (status) {
    case 'critical':
      return 'text-status-failed-text'
    case 'degraded':
      return 'text-status-warning-text'
    case 'ok':
    default:
      return 'text-status-ok-text'
  }
}

export function getHealthBadgeVariant(status: 'ok' | 'degraded' | 'critical'): 'default' | 'destructive' | 'secondary' {
  switch (status) {
    case 'critical':
      return 'destructive'
    case 'degraded':
      return 'secondary'
    case 'ok':
    default:
      return 'default'
  }
}
