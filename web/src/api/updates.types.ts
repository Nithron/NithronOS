export type UpdateChannel = 'stable' | 'beta'

export type UpdateState =
  | 'idle'
  | 'checking'
  | 'downloading'
  | 'applying'
  | 'verifying'
  | 'success'
  | 'failed'
  | 'rolling_back'
  | 'rolled_back'

export type UpdatePhase =
  | 'preflight'
  | 'snapshot'
  | 'download'
  | 'install'
  | 'postflight'
  | 'cleanup'
  | 'rollback'

export interface UpdateConfig {
  /** How many update snapshots are kept; older ones go after each successful update. */
  snapshot_retention: number
}

export interface SystemVersion {
  os_version: string
  kernel: string
  nosd_version: string
  agent_version: string
  webui_version: string
  channel: UpdateChannel
  commit?: string
  build_date?: string
}

export interface Package {
  name: string
  current_version: string
  new_version: string
  size?: number
  signature?: string
}

export interface AvailableUpdate {
  version: string
  channel: UpdateChannel
  release_date: string
  size: number
  changelog_url?: string
  packages: Package[]
  critical: boolean
  requires_reboot: boolean
}

export interface UpdateProgress {
  state: UpdateState
  phase: UpdatePhase
  progress: number // 0-100
  message: string
  started_at: string
  completed_at?: string
  estimated_time_remaining?: number // seconds
  logs: LogEntry[]
  error?: string
  snapshot_id?: string
}

export interface LogEntry {
  timestamp: string
  level: 'info' | 'warn' | 'error'
  message: string
  phase?: UpdatePhase
}

export interface UpdateSnapshot {
  id: string
  version?: SystemVersion
  created_at: string
  reason: 'update' | 'manual' | 'automatic'
  size: number
  subvolumes: string[]
  can_rollback: boolean
  description?: string
}

export interface UpdateCheckResponse {
  update_available: boolean
  current_version: SystemVersion
  latest_version?: AvailableUpdate
  last_check: string
}

export interface ChannelChangeRequest {
  channel: UpdateChannel
}

export interface UpdateApplyRequest {
  version?: string
  skip_snapshot?: boolean
  force?: boolean
}

export interface RollbackRequest {
  snapshot_id: string
  force?: boolean
}

export interface PreflightCheck {
  check_type: 'disk_space' | 'network' | 'repo' | 'signature'
  status: 'pass' | 'fail' | 'warning'
  message: string
  required: boolean
}

export interface PostflightCheck {
  service: string
  status: 'running' | 'stopped' | 'degraded'
  healthy: boolean
  message: string
  critical: boolean
}
