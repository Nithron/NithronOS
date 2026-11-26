import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { endpoints } from '@/lib/api'

// TODO: Move these types to a central types file
export interface ScrubStatus {
  running: boolean;
  progress?: number;
  eta?: string;
  status?: string;
  poolId?: string;
  nextRun?: string;
}

export interface BalanceStatus {
  running: boolean;
  progress?: number;
  eta?: string;
  status?: string;
  poolId?: string;
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
  description?: string;
  guestOk?: boolean;
  readOnly?: boolean;
  users?: string[];
  groups?: string[];
}

// ============================================================================
// System Queries
// ============================================================================

export function useUsers() {
  return useQuery({
    queryKey: ['users'],
    queryFn: () => ({ data: [] }),
  })
}

export function useApps() {
  return useQuery({
    queryKey: ['apps'],
    queryFn: () => ({ apps: [] }),
  })
}

export function useMarketplace() {
  return useQuery({
    queryKey: ['marketplace'],
    queryFn: () => ({ apps: [] }),
  })
}

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system', 'info'],
    queryFn: async () => {
      const data = await endpoints.system.info()
      return data as any
    },
    staleTime: 10_000,
    retry: 1,
  })
}

export function useSystemMetrics() {
  return useQuery({
    queryKey: ['system', 'metrics'],
    queryFn: endpoints.system.metrics,
    staleTime: 5_000,
    refetchInterval: 5_000,
    retry: 1,
  })
}

export function useServices() {
  return useQuery({
    queryKey: ['system', 'services'],
    queryFn: endpoints.system.services,
    staleTime: 5_000,
    retry: 1,
  })
}

// ============================================================================
// Storage Pool Queries (M1)
// ============================================================================

export function usePools() {
  return useQuery({
    queryKey: ['pools'],
    queryFn: async () => {
      const data = await endpoints.pools.list()
      return (data as any[]) || []
    },
    staleTime: 10_000,
    retry: 1,
  })
}

export function usePoolsSummary() {
  return useQuery({
    queryKey: ['pools', 'summary'],
    queryFn: endpoints.pools.summary,
    staleTime: 10_000,
    retry: 1,
  })
}

export function usePool(uuid: string) {
  return useQuery({
    queryKey: ['pools', uuid],
    queryFn: () => endpoints.pools.get(uuid),
    enabled: !!uuid,
    staleTime: 10_000,
    retry: 1,
  })
}

export function usePoolSubvolumes(uuid: string) {
  return useQuery({
    queryKey: ['pools', uuid, 'subvolumes'],
    queryFn: () => endpoints.pools.subvolumes(uuid),
    enabled: !!uuid,
    staleTime: 10_000,
    retry: 1,
  })
}

export function usePoolMountOptions(uuid: string) {
  return useQuery({
    queryKey: ['pools', uuid, 'mount-options'],
    queryFn: () => endpoints.pools.getMountOptions(uuid),
    enabled: !!uuid,
    staleTime: 30_000,
    retry: 1,
  })
}

export function useUpdatePoolMountOptions() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ uuid, options }: { uuid: string; options: string }) =>
      endpoints.pools.setMountOptions(uuid, options),
    onSuccess: (_, { uuid }) => {
      queryClient.invalidateQueries({ queryKey: ['pools', uuid, 'mount-options'] })
    },
  })
}

// ============================================================================
// Storage Device Queries
// ============================================================================

export function useDevices() {
  return useQuery({
    queryKey: ['devices'],
    queryFn: async () => {
      const data = await endpoints.devices.list()
      return (data as any[]) || []
    },
    staleTime: 10_000,
    retry: 1,
  })
}

// ============================================================================
// SMART Health Queries (M1)
// ============================================================================

export function useSmartSummary() {
  return useQuery({
    queryKey: ['smart', 'summary'],
    queryFn: endpoints.smart.summary,
    staleTime: 10_000,
    retry: 1,
  })
}

export function useSmartDevice(device: string) {
  return useQuery({
    queryKey: ['smart', device],
    queryFn: async () => {
      const data = await endpoints.smart.device(device)
      return data as any
    },
    enabled: !!device,
    staleTime: 10_000,
    retry: 1,
  })
}

export function useSmartScan() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.smart.scan,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['smart'] })
    },
  })
}

export function useSmartDevices() {
  return useQuery({
    queryKey: ['smart', 'devices'],
    queryFn: endpoints.smart.devices,
    staleTime: 10_000,
    retry: 1,
  })
}

export function useSmartTest(device: string) {
  return useQuery({
    queryKey: ['smart', 'test', device],
    queryFn: () => endpoints.smart.test(device),
    enabled: !!device,
    staleTime: 5_000,
    retry: 1,
  })
}

export function useRunSmartTest() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ device, type }: { device: string; type: 'short' | 'long' | 'conveyance' }) =>
      endpoints.smart.runTest(device, type),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['smart'] })
    },
  })
}

// ============================================================================
// Scrub Queries (M1)
// ============================================================================

export function useScrubStatus() {
  return useQuery({
    queryKey: ['scrub', 'status'],
    queryFn: async () => {
      const data = await endpoints.scrub.status()
      return (data as ScrubStatus[]) || []
    },
    staleTime: 5_000,
    retry: 1,
    refetchInterval: (data) => {
      // Poll more frequently if scrub is running
      const hasRunning = Array.isArray(data) && data.some((s: ScrubStatus) => s.status === 'running')
      return hasRunning ? 5_000 : false
    },
  })
}

export function useStartScrub() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.scrub.start,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scrub'] })
    },
  })
}

export function useCancelScrub() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.scrub.cancel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['scrub'] })
    },
  })
}

// ============================================================================
// Balance Queries (M1)
// ============================================================================

export function useBalanceStatus() {
  return useQuery({
    queryKey: ['balance', 'status'],
    queryFn: async () => {
      const data = await endpoints.balance.status()
      return (data as BalanceStatus[]) || []
    },
    staleTime: 5_000,
    retry: 1,
    refetchInterval: (data) => {
      // Poll more frequently if balance is running
      const hasRunning = Array.isArray(data) && data.some((b: BalanceStatus) => b.status === 'running')
      return hasRunning ? 5_000 : false
    },
  })
}

export function useStartBalance() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.balance.start,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['balance'] })
    },
  })
}

export function useCancelBalance() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.balance.cancel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['balance'] })
    },
  })
}

// ============================================================================
// Schedule Queries (M1)
// ============================================================================

export function useSchedules() {
  return useQuery({
    queryKey: ['schedules'],
    queryFn: endpoints.schedules.list,
    staleTime: 30_000,
    retry: 1,
  })
}

export function useCreateSchedule() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.schedules.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })
}

export function useUpdateSchedule() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ id, schedule }: { id: string; schedule: Partial<Schedule> }) =>
      endpoints.schedules.update(id, schedule),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })
}

export function useDeleteSchedule() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.schedules.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['schedules'] })
    },
  })
}

// ============================================================================
// Job Queries
// ============================================================================

export function useRecentJobs(limit = 10) {
  return useQuery({
    queryKey: ['jobs', 'recent', limit],
    queryFn: () => endpoints.jobs.recent(limit),
    staleTime: 5_000,
    retry: 1,
  })
}

// ============================================================================
// Share Queries (M2)
// ============================================================================

export function useShares() {
  return useQuery({
    queryKey: ['shares'],
    queryFn: async () => {
      const data = await endpoints.shares.list()
      return (data as Share[]) || []
    },
    staleTime: 10_000,
    retry: 1,
  })
}

export function useShare(name: string) {
  return useQuery({
    queryKey: ['shares', name],
    queryFn: () => endpoints.shares.get(name),
    enabled: !!name,
    staleTime: 10_000,
    retry: 1,
  })
}

export function useCreateShare() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.shares.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shares'] })
    },
  })
}

export function useUpdateShare() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: ({ name, share }: { name: string; share: Partial<Share> }) =>
      endpoints.shares.update(name, share),
    onSuccess: (_, { name }) => {
      queryClient.invalidateQueries({ queryKey: ['shares'] })
      queryClient.invalidateQueries({ queryKey: ['shares', name] })
    },
  })
}

export function useDeleteShare() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.shares.delete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['shares'] })
    },
  })
}

export function useTestShare() {
  return useMutation({
    mutationFn: ({ name, config }: { name: string; config: any }) =>
      endpoints.shares.test(name, config),
  })
}

// ============================================================================
// App Queries (M3)
// ============================================================================

export function useAppCatalog() {
  return useQuery({
    queryKey: ['apps', 'catalog'],
    queryFn: async () => {
      const data = await endpoints.apps.catalog()
      return (data as any).entries || []
    },
    staleTime: 60_000,
    retry: 1,
  })
}

export function useInstalledApps() {
  return useQuery({
    queryKey: ['apps', 'installed'],
    queryFn: async () => {
      const data = await endpoints.apps.installed()
      return (data as any).items || []
    },
    staleTime: 10_000,
    retry: 1,
  })
}

export function useApp(id: string) {
  return useQuery({
    queryKey: ['apps', id],
    queryFn: () => endpoints.apps.get(id),
    enabled: !!id,
    staleTime: 5_000,
    retry: 1,
  })
}

export function useInstallApp() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: endpoints.apps.install,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apps'] })
    },
  })
}

export function useAppActions() {
  const queryClient = useQueryClient()
  
  return {
    start: useMutation({
      mutationFn: endpoints.apps.start,
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['apps'] })
      },
    }),
    stop: useMutation({
      mutationFn: endpoints.apps.stop,
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['apps'] })
      },
    }),
    restart: useMutation({
      mutationFn: endpoints.apps.restart,
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['apps'] })
      },
    }),
    upgrade: useMutation({
      mutationFn: ({ id, params }: { id: string; params?: Record<string, any> }) =>
        endpoints.apps.upgrade(id, params),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['apps'] })
      },
    }),
    delete: useMutation({
      mutationFn: ({ id, keepData }: { id: string; keepData: boolean }) =>
        endpoints.apps.delete(id, keepData),
      onSuccess: () => {
        queryClient.invalidateQueries({ queryKey: ['apps'] })
      },
    }),
  }
}

// ============================================================================
// Error Handling Helper
// ============================================================================

export function useApiErrorHandler() {
  const queryClient = useQueryClient()
  
  return {
    handleError: (error: any) => {
      // Check for proxy error
      if (error?.status === 502 || error?.message?.includes('Backend unreachable')) {
        // Set global error state that UI can check
        queryClient.setQueryData(['api', 'status'], { 
          isReachable: false,
          error: 'Backend unreachable or proxy misconfigured' 
        })
        return
      }
      
      // Check for auth error
      if (error?.status === 401) {
        // Redirect to login
        window.location.href = '/login'
        return
      }
      
      // Log other errors
      console.error('API Error:', error)
    },
    
    clearError: () => {
      queryClient.setQueryData(['api', 'status'], { isReachable: true })
    }
  }
}

export function useApiStatus() {
  return useQuery({
    queryKey: ['api', 'status'],
    queryFn: async () => {
      // Check if API is reachable
      try {
        await endpoints.system.info()
        return { isReachable: true }
      } catch (error: any) {
        if (error?.status === 502 || error?.message?.includes('Backend unreachable')) {
          return { 
            isReachable: false, 
            error: 'Backend unreachable or proxy misconfigured' 
          }
        }
        throw error
      }
    },
    staleTime: 30_000,
    retry: false,
  })
}

// ============================================================================
// Monitoring Queries
// ============================================================================

export function useSystemLogs(filter?: { level?: string; service?: string; limit?: number }) {
  return useQuery({
    queryKey: ['monitoring', 'logs', filter],
    queryFn: () => endpoints.monitoring?.getLogs?.() || Promise.resolve([]),
    staleTime: 5_000,
    refetchInterval: 5_000,
    retry: 1,
  })
}

export function useSystemEvents(limit = 100) {
  return useQuery({
    queryKey: ['monitoring', 'events', limit],
    queryFn: () => endpoints.monitoring?.getEvents?.() || Promise.resolve([]),
    staleTime: 5_000,
    refetchInterval: 5_000,
    retry: 1,
  })
}

export function useSystemAlerts() {
  return useQuery({
    queryKey: ['monitoring', 'alerts'],
    queryFn: endpoints.monitoring?.getAlerts || (() => Promise.resolve([])),
    staleTime: 5_000,
    refetchInterval: 10_000,
    retry: 1,
  })
}

export function useServiceStatus() {
  return useQuery({
    queryKey: ['monitoring', 'services'],
    queryFn: endpoints.monitoring?.getServiceStatus || (() => Promise.resolve([])),
    staleTime: 5_000,
    refetchInterval: 10_000,
    retry: 1,
  })
}