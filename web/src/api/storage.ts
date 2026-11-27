import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import http from '@/lib/nos-client'

/** A Btrfs pool as GET /api/v1/pools lists it. */
export interface Pool {
  /** uuid or label */
  id: string
  label: string
  uuid: string
  /** where it is mounted; absent when it isn't */
  mount?: string
  devices: string[]
  size: number
  used: number
  free: number
  raid: string
}

/** A block device as GET /api/v1/devices lists it (pool candidates). */
export interface Device {
  name: string
  path: string
  size: number
  model?: string
  serial?: string
  /** rotational: a spinning disk */
  rota?: boolean
  fsType?: string
  btrfsMember: boolean
  luks: boolean
  warnings: string[]
}

export type MaintenanceState =
  | 'idle'
  | 'running'
  | 'finished'
  | 'aborted'
  | 'interrupted'
  | 'unknown'

/** One pool's scrub, from GET /api/v1/scrub/status. */
export interface ScrubStatus {
  poolId: string
  mount: string
  status: MaintenanceState
  progress?: number
  bytesScanned?: number
  bytesTotal?: number
  errors?: number
  startedAt?: string
  finishedAt?: string
  error?: string
}

/** One pool's balance, from GET /api/v1/balance/status. */
export interface BalanceStatus {
  poolId: string
  mount: string
  status: 'idle' | 'running' | 'unknown'
  progress?: number
  left?: string
  total?: string
  error?: string
}

/** A disk's SMART state, from GET /api/v1/smart/device/{name}. */
export interface SmartDevice {
  device: string
  model: string
  serial_number: string
  capacity_bytes: number
  temperature_celsius?: number
  power_on_hours?: number
  health: 'good' | 'warning' | 'critical' | 'unknown'
  last_checked: string
}

export const storageApi = {
  pools: () => http.get<Pool[]>('/v1/pools'),
  devices: async () => (await http.get<{ devices: Device[] }>('/v1/devices')).devices ?? [],
  scrubStatus: () => http.get<ScrubStatus[]>('/v1/scrub/status'),
  balanceStatus: () => http.get<BalanceStatus[]>('/v1/balance/status'),
  /** Starts a scrub of one pool. */
  startScrub: (poolId: string) => http.post('/v1/scrub/start', { poolId }),
  /** Starts a balance of one pool. */
  startBalance: (poolId: string) => http.post('/v1/balance/start', { poolId }),
  /** Disks for a new pool (GET /api/v1/disks). */
  disks: async () =>
    (
      await http.get<{
        disks: Array<{ name: string; path: string; size: number; model?: string; type?: string }>
      }>('/v1/disks')
    ).disks ?? [],
  /** Plans a new pool: the commands it would run. */
  planCreate: (body: unknown) =>
    http.post<{ plan: unknown; fstab?: unknown }>('/v1/pools/plan-create', body),
  /** Runs a planned pool creation (confirm: "CREATE"). */
  applyCreate: (body: unknown) => http.post<unknown>('/v1/pools/apply-create', body),
  /** name: the kernel name, e.g. "sda" (not /dev/sda). */
  smartDevice: (name: string) =>
    http.get<SmartDevice>(`/v1/smart/device/${encodeURIComponent(name)}`),
}

const running = (list?: Array<{ status: string }>) =>
  (list ?? []).some((s) => s.status === 'running')

export function usePools() {
  return useQuery({ queryKey: ['pools'], queryFn: storageApi.pools })
}

export function useDevices() {
  return useQuery({ queryKey: ['devices'], queryFn: storageApi.devices })
}

/** Polls every 5 s while a scrub runs. */
export function useScrubStatus() {
  return useQuery({
    queryKey: ['scrub', 'status'],
    queryFn: storageApi.scrubStatus,
    refetchInterval: (q) => (running(q.state.data) ? 5000 : false),
  })
}

/** Polls every 5 s while a balance runs. */
export function useBalanceStatus() {
  return useQuery({
    queryKey: ['balance', 'status'],
    queryFn: storageApi.balanceStatus,
    refetchInterval: (q) => (running(q.state.data) ? 5000 : false),
  })
}

export function useSmartDevice(name: string | null) {
  return useQuery({
    queryKey: ['smart', name],
    queryFn: () => storageApi.smartDevice(name!),
    enabled: !!name,
    staleTime: 60_000,
  })
}

export function useMaintenanceMutations() {
  const qc = useQueryClient()
  return {
    startScrub: useMutation({
      mutationFn: storageApi.startScrub,
      onSuccess: () => qc.invalidateQueries({ queryKey: ['scrub'] }),
    }),
    startBalance: useMutation({
      mutationFn: storageApi.startBalance,
      onSuccess: () => qc.invalidateQueries({ queryKey: ['balance'] }),
    }),
  }
}
