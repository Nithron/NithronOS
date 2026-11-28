import { useState, useEffect } from 'react'
import { formatDate, formatDateTime, formatTime } from '@/lib/datetime'
import { confirmAction } from '@/components/ui/confirm'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { motion } from 'framer-motion'
import {
  AppIcon,
  ChannelIcon,
  DeleteIcon,
  DownloadIcon,
  RefreshIcon,
  ServerIcon,
  SnapshotIcon,
  TimeIcon,
  WarningIcon,
  WorkingIcon,
} from '@/components/icons'
import { toast } from '@/components/ui/toast'
import { Meter, StatusMark } from '@/components/ui/pixel'

import {
  updatesApi,
  formatVersion,
  getUpdateStateColor,
  formatBytes,
  formatDuration,
} from '@/api/updates'
import type {
  UpdateProgress,
  UpdateSnapshot,
  UpdateChannel,
  UpdateState,
} from '@/api/updates.types'

// States in which an update or a rollback is still running.
const runningStates: UpdateState[] = [
  'checking',
  'downloading',
  'applying',
  'verifying',
  'rolling_back',
]

// The phases nosd reports, in order, and the progress at which each is done.
const phases = [
  { id: 'preflight', name: 'Checks', doneAt: 8 },
  { id: 'snapshot', name: 'Snapshot', doneAt: 20 },
  { id: 'install', name: 'Install', doneAt: 92 },
  { id: 'postflight', name: 'Verify', doneAt: 100 },
] as const

export default function Updates() {
  const queryClient = useQueryClient()
  const [selectedChannel, setSelectedChannel] = useState<UpdateChannel>('stable')
  const [updateProgress, setUpdateProgress] = useState<UpdateProgress | null>(null)
  const [eventSource, setEventSource] = useState<EventSource | null>(null)
  const [showSnapshots, setShowSnapshots] = useState(false)
  const busy = updateProgress !== null && runningStates.includes(updateProgress.state)

  // Queries
  const { data: version } = useQuery({
    queryKey: ['updates', 'version'],
    queryFn: updatesApi.getVersion,
  })

  const { data: channelData } = useQuery({
    queryKey: ['updates', 'channel'],
    queryFn: updatesApi.getChannel,
  })

  useEffect(() => {
    if (channelData) {
      setSelectedChannel(channelData.channel)
    }
  }, [channelData])

  const {
    data: updateCheck,
    isLoading: isChecking,
    refetch: checkForUpdates,
  } = useQuery({
    queryKey: ['updates', 'check'],
    queryFn: updatesApi.checkForUpdates,
    refetchInterval: false,
  })

  const { data: updateConfig } = useQuery({
    queryKey: ['updates', 'config'],
    queryFn: updatesApi.getConfig,
  })

  const configMutation = useMutation({
    mutationFn: updatesApi.setConfig,
    onSuccess: (config) => {
      toast.success(`The newest ${config.snapshot_retention} snapshots are kept`)
      queryClient.invalidateQueries({ queryKey: ['updates', 'config'] })
    },
    onError: (error: Error) => toast.error(error.message || 'The setting could not be saved'),
  })

  const { data: snapshots, refetch: refetchSnapshots } = useQuery({
    queryKey: ['updates', 'snapshots'],
    queryFn: updatesApi.listSnapshots,
    enabled: showSnapshots,
  })

  // Mutations
  const changeChannelMutation = useMutation({
    mutationFn: updatesApi.setChannel,
    onSuccess: () => {
      toast.success('Update channel changed')
      queryClient.invalidateQueries({ queryKey: ['updates'] })
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to change channel')
    },
  })

  const applyUpdateMutation = useMutation({
    mutationFn: updatesApi.applyUpdate,
    onSuccess: () => {
      toast.success('Update started')
      startProgressStream()
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to start update')
    },
  })

  const rollbackMutation = useMutation({
    mutationFn: updatesApi.rollback,
    onSuccess: () => {
      toast.success('Rollback started')
      startProgressStream()
    },
    onError: (error: any) => {
      toast.error(error.message || 'Rollback failed')
    },
  })

  const deleteSnapshotMutation = useMutation({
    mutationFn: updatesApi.deleteSnapshot,
    onSuccess: () => {
      toast.success('Snapshot deleted')
      refetchSnapshots()
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to delete snapshot')
    },
  })

  // Start progress streaming
  const startProgressStream = () => {
    if (eventSource) {
      eventSource.close()
    }

    const source = updatesApi.streamProgress((progress) => {
      setUpdateProgress(progress)

      // Check if update is complete
      if (
        progress.state === 'success' ||
        progress.state === 'failed' ||
        progress.state === 'rolled_back'
      ) {
        source.close()
        setEventSource(null)
        queryClient.invalidateQueries({ queryKey: ['updates'] })

        if (progress.state === 'success') {
          toast.success(progress.message || 'Update installed')
        } else if (progress.state === 'failed') {
          toast.error(progress.error || 'The update failed')
        } else if (progress.error) {
          toast.error(`The update failed and was rolled back: ${progress.error}`)
        } else {
          toast.success('Rolled back')
        }
      }
    })

    setEventSource(source)
  }

  // Check for ongoing update on mount
  useEffect(() => {
    updatesApi.getProgress().then((progress: UpdateProgress | null) => {
      if (progress && progress.state !== 'idle') {
        setUpdateProgress(progress)
        if (runningStates.includes(progress.state)) {
          startProgressStream()
        }
      }
    })

    return () => {
      if (eventSource) {
        eventSource.close()
      }
    }
  }, [])

  const handleChannelChange = (channel: UpdateChannel) => {
    setSelectedChannel(channel)
    changeChannelMutation.mutate({ channel })
  }

  const handleApplyUpdate = () => {
    if (updateCheck?.latest_version) {
      applyUpdateMutation.mutate({
        version: updateCheck.latest_version.version,
      })
    }
  }

  // A rollback restores the configuration saved before an update
  // (/etc/nos), reinstalls the package versions installed then, and
  // restarts nosd.
  const handleRollback = async (snapshotId: string) => {
    if (
      await confirmAction({
        title: 'Roll back this update?',
        description:
          'nOS puts back its configuration as it was before that update and reinstalls the package versions installed then. nosd restarts, so this page reconnects.',
        action: 'Roll back',
      })
    ) {
      rollbackMutation.mutate({ snapshot_id: snapshotId })
    }
  }

  const handleDeleteSnapshot = async (snapshotId: string) => {
    if (
      await confirmAction({
        title: 'Delete this snapshot?',
        description: 'It cannot be restored afterwards.',
        action: 'Delete snapshot',
      })
    ) {
      deleteSnapshotMutation.mutate(snapshotId)
    }
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="screen-title">System updates</h1>
          <p className="text-ink-2 mt-1">Manage system updates and snapshots</p>
        </div>
        <button
          onClick={() => checkForUpdates()}
          disabled={isChecking || busy}
          className="btn btn-primary"
        >
          {isChecking ? <WorkingIcon className="h-4 w-4" /> : <RefreshIcon className="h-4 w-4" />}
          {isChecking ? 'Checking' : 'Check for updates'}
        </button>
      </div>

      {/* Current Version Card */}
      <div className="border border-hairline p-6">
        <h2 className="mb-4 text-xl font-[560]">Current version</h2>
        {version && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="flex items-start gap-3">
              <ServerIcon className="w-5 h-5 text-ink-2 mt-1" />
              <div>
                <p className="text-sm text-ink-2">OS version</p>
                <p className="text-ink font-medium">{version.os_version}</p>
                <p className="text-xs text-ink-2 mt-1">{version.kernel}</p>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <AppIcon className="w-5 h-5 text-ink-2 mt-1" />
              <div>
                <p className="text-sm text-ink-2">Components</p>
                <p className="text-ink font-medium">{formatVersion(version)}</p>
                {version.build_date && (
                  <p className="text-xs text-ink-2 mt-1">Built {formatDate(version.build_date)}</p>
                )}
              </div>
            </div>

            <div className="flex items-start gap-3">
              <ChannelIcon className="w-5 h-5 text-ink-2 mt-1" />
              <div>
                <p className="text-sm text-ink-2">Update channel</p>
                <div className="flex items-center gap-2 mt-1">
                  <select
                    value={selectedChannel}
                    onChange={(e) => handleChannelChange(e.target.value as UpdateChannel)}
                    className="field h-8 w-auto"
                    disabled={busy}
                  >
                    <option value="stable">Stable</option>
                    <option value="beta">Beta</option>
                  </select>
                  {selectedChannel === 'beta' && (
                    <span className="flex items-center gap-1 text-xs text-ink-2">
                      <WarningIcon className="h-4 w-4 text-status-warning" />
                      May be unstable
                    </span>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Available Update Card */}
      {updateCheck?.update_available && updateCheck.latest_version && (
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          className="border border-ink p-6"
        >
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center gap-2 mb-3">
                <DownloadIcon className="h-4 w-4 text-ink" />
                <h2 className="text-xl font-[560]">Update available</h2>
                {updateCheck.latest_version.critical && (
                  <span className="rounded-sm border border-rule px-2 py-0.5 text-xs text-ink">
                    Critical
                  </span>
                )}
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <p className="text-sm text-ink-2">Version</p>
                  <p className="text-ink font-medium">{updateCheck.latest_version.version}</p>
                </div>
                <div>
                  <p className="text-sm text-ink-2">Release date</p>
                  <p className="text-ink font-medium">
                    {formatDate(updateCheck.latest_version.release_date)}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-ink-2">Size</p>
                  <p className="text-ink font-medium">
                    {formatBytes(updateCheck.latest_version.size)}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-ink-2">Packages</p>
                  <p className="text-ink font-medium">
                    {updateCheck.latest_version.packages.length} packages
                  </p>
                </div>
              </div>

              {updateCheck.latest_version.requires_reboot && (
                <p className="mb-4 flex items-center gap-2 text-sm text-ink">
                  <WarningIcon className="h-4 w-4 text-status-warning" />
                  The system restarts to finish this update
                </p>
              )}
            </div>

            <button
              onClick={handleApplyUpdate}
              disabled={busy || applyUpdateMutation.isPending}
              className="btn btn-primary"
            >
              {applyUpdateMutation.isPending ? 'Starting' : 'Apply update'}
            </button>
          </div>

          {updateCheck.latest_version.changelog_url && (
            <a
              href={updateCheck.latest_version.changelog_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-ink underline decoration-ink-2 underline-offset-4 hover:decoration-ink"
            >
              View Changelog →
            </a>
          )}
        </motion.div>
      )}

      {/* Update Progress */}
      {updateProgress && updateProgress.state !== 'idle' && (
        <motion.div
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          className="border border-hairline p-6"
        >
          <h2 className="mb-4 text-xl font-[560]">Update progress</h2>

          <div className="space-y-4">
            {/* Progress Bar */}
            <div>
              <p className={`mb-2 text-sm ${getUpdateStateColor(updateProgress.state)}`}>
                {updateProgress.message}
              </p>
              <Meter
                value={updateProgress.progress}
                max={100}
                kind="progress"
                label="Update progress"
              />
            </div>

            {/* The phases: done, the current one (a lamp), still to come. A
                rollback has a phase of its own. */}
            {updateProgress.phase === 'rollback' ? (
              <StatusMark
                state={
                  updateProgress.state === 'rolling_back'
                    ? 'running'
                    : updateProgress.state === 'rolled_back'
                      ? 'ok'
                      : 'failed'
                }
                className="text-sm font-[560] text-ink"
              >
                Rollback
              </StatusMark>
            ) : (
              <ol className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
                {phases.map((phase) => {
                  const state =
                    updateProgress.phase === phase.id && busy
                      ? 'running'
                      : updateProgress.progress >= phase.doneAt
                        ? 'ok'
                        : updateProgress.phase === phase.id && updateProgress.state === 'failed'
                          ? 'failed'
                          : 'off'
                  return (
                    <li key={phase.id} aria-current={state === 'running' ? 'step' : undefined}>
                      <StatusMark
                        state={state}
                        className={state === 'running' ? 'font-[560] text-ink' : 'text-ink-2'}
                      >
                        {phase.name}
                      </StatusMark>
                    </li>
                  )
                })}
              </ol>
            )}
            {updateProgress.error && (
              <p role="alert" className="text-sm text-ink">
                {updateProgress.error}
              </p>
            )}

            {/* Logs */}
            {updateProgress.logs.length > 0 && (
              <div className="max-h-40 overflow-y-auto border border-hairline p-3 font-mono text-xs">
                {updateProgress.logs.slice(-5).map((log, i) => (
                  <div key={i} className="flex items-start gap-2 text-xs">
                    <span className="text-ink-2">{formatTime(log.timestamp)}</span>
                    <span
                      className={
                        log.level === 'error'
                          ? 'text-status-failed-text'
                          : log.level === 'warn'
                            ? 'text-status-warning-text'
                            : 'text-ink-2'
                      }
                    >
                      {log.message}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {/* ETA */}
            {updateProgress.estimated_time_remaining && (
              <div className="flex items-center gap-2 text-sm text-ink-2">
                <TimeIcon className="w-4 h-4" />
                <span>
                  Estimated time remaining:{' '}
                  {formatDuration(updateProgress.estimated_time_remaining)}
                </span>
              </div>
            )}
          </div>
        </motion.div>
      )}

      {/* Snapshots Section */}
      <div className="border border-hairline p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-[560]">System snapshots</h2>
          <button
            onClick={() => setShowSnapshots(!showSnapshots)}
            className="text-sm text-ink underline decoration-ink-2 underline-offset-4 hover:decoration-ink"
          >
            {showSnapshots ? 'Hide' : 'Show'} snapshots
          </button>
        </div>

        <div className="mb-4 flex flex-wrap items-center gap-3 text-sm">
          <label htmlFor="snapshot-retention">Snapshots kept after each update</label>
          <select
            id="snapshot-retention"
            className="field h-8 w-auto"
            value={updateConfig?.snapshot_retention ?? 3}
            disabled={!updateConfig || configMutation.isPending}
            onChange={(e) => configMutation.mutate({ snapshot_retention: Number(e.target.value) })}
          >
            {[1, 2, 3, 5, 10, 20].map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
          <span className="text-ink-2">Older ones are deleted when an update succeeds.</span>
        </div>

        {showSnapshots && snapshots && (
          <div className="space-y-3">
            {snapshots.snapshots.length === 0 ? (
              <p className="py-8 text-ink-2">No snapshots available</p>
            ) : (
              snapshots.snapshots.map((snapshot: UpdateSnapshot) => (
                <div
                  key={snapshot.id}
                  className="flex items-center justify-between border-b border-hairline py-4"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <SnapshotIcon className="w-5 h-5 text-ink-2" />
                      <div>
                        <p className="text-ink font-medium">{snapshot.id}</p>
                        <p className="text-sm text-ink-2">
                          {formatDateTime(snapshot.created_at)} • {formatBytes(snapshot.size)}
                        </p>
                        {snapshot.description && (
                          <p className="text-sm text-ink-2 mt-1">{snapshot.description}</p>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {snapshot.can_rollback && (
                      <button
                        onClick={() => handleRollback(snapshot.id)}
                        disabled={busy}
                        className="p-2 text-ink-2 hover:bg-ink/[.09] hover:text-ink"
                      >
                        Roll back
                      </button>
                    )}
                    <button
                      onClick={() => handleDeleteSnapshot(snapshot.id)}
                      className="p-2 text-ink-2 hover:bg-ink/[.09] hover:text-ink"
                      title="Delete snapshot"
                    >
                      <DeleteIcon className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  )
}
