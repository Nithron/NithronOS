// Health API hooks with 1Hz refresh
import { useQuery } from '@tanstack/react-query'
import { healthApi, type SystemHealth, type DiskHealth } from '@/lib/api-health'

// Hook for system health with 1Hz refresh
export function useSystemHealth() {
  return useQuery<SystemHealth>({
    queryKey: ['health', 'system'],
    queryFn: healthApi.getSystemHealth,
    refetchInterval: 1000, // 1Hz refresh
    refetchIntervalInBackground: true,
    staleTime: 500, // Consider data stale after 500ms
    gcTime: 60000, // Keep in cache for 1 minute (was cacheTime)
    retry: 2,
    retryDelay: 1000,
  })
}

// Hook for disk health with 1Hz refresh
export function useDiskHealth() {
  return useQuery<DiskHealth[]>({
    queryKey: ['health', 'disks'],
    queryFn: healthApi.getDiskHealth,
    refetchInterval: 1000, // 1Hz refresh
    refetchIntervalInBackground: true,
    staleTime: 500, // Consider data stale after 500ms
    gcTime: 60000, // Keep in cache for 1 minute
    retry: 1,
    retryDelay: 1000,
    // Always return an array to prevent crashes
    select: (data) => Array.isArray(data) ? data : [],
  })
}

// Hook for monitoring data with 1Hz refresh
export function useMonitoringData() {
  return useQuery<SystemHealth>({
    queryKey: ['monitor', 'system'],
    queryFn: healthApi.getMonitoringData,
    refetchInterval: 1000, // 1Hz refresh
    refetchIntervalInBackground: true,
    staleTime: 500, // Consider data stale after 500ms
    gcTime: 60000, // Keep in cache for 1 minute
    retry: 2,
    retryDelay: 1000,
  })
}

// Helper function to format bytes
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`
}

// Helper function to format uptime
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

// Helper function to format percentage with animation
export function formatPercent(value: number): string {
  return `${Math.round(value)}%`
}

// Helper to determine health status based on metrics
export function getHealthStatus(cpu: number, memory: number, disk: number): 'healthy' | 'warning' | 'critical' {
  if (cpu > 90 || memory > 90 || disk > 90) return 'critical'
  if (cpu > 70 || memory > 70 || disk > 80) return 'warning'
  return 'healthy'
}

// Helper to determine disk health color
export function getDiskHealthColor(state: string): string {
  switch (state) {
    case 'critical':
      return 'text-status-failed-text'
    case 'warning':
      return 'text-status-warning-text'
    case 'healthy':
    default:
      return 'text-status-ok-text'
  }
}
