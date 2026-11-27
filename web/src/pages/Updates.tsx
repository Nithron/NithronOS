import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { motion } from 'framer-motion';
import { AppIcon, ChannelIcon, DeleteIcon, DownloadIcon, OkIcon, RefreshIcon, RollBackIcon, ServerIcon, SnapshotIcon, TimeIcon, WorkingIcon } from '@/components/icons'
import { format } from 'date-fns';
import { toast } from 'react-hot-toast';

import { updatesApi, formatVersion, getUpdateStateColor, formatBytes, formatDuration } from '@/api/updates';
import type { UpdateProgress, UpdateSnapshot, UpdateChannel } from '@/api/updates.types';

export default function Updates() {
  const queryClient = useQueryClient();
  const [selectedChannel, setSelectedChannel] = useState<UpdateChannel>('stable');
  const [updateProgress, setUpdateProgress] = useState<UpdateProgress | null>(null);
  const [eventSource, setEventSource] = useState<EventSource | null>(null);
  const [showSnapshots, setShowSnapshots] = useState(false);

  // Queries
  const { data: version } = useQuery({
    queryKey: ['updates', 'version'],
    queryFn: updatesApi.getVersion,
  });

  const { data: channelData } = useQuery({
    queryKey: ['updates', 'channel'],
    queryFn: updatesApi.getChannel,
  });

  useEffect(() => {
    if (channelData) {
      setSelectedChannel(channelData.channel);
    }
  }, [channelData]);

  const { data: updateCheck, isLoading: isChecking, refetch: checkForUpdates } = useQuery({
    queryKey: ['updates', 'check'],
    queryFn: updatesApi.checkForUpdates,
    refetchInterval: false,
  });

  const { data: snapshots, refetch: refetchSnapshots } = useQuery({
    queryKey: ['updates', 'snapshots'],
    queryFn: updatesApi.listSnapshots,
    enabled: showSnapshots,
  });

  // Mutations
  const changeChannelMutation = useMutation({
    mutationFn: updatesApi.setChannel,
    onSuccess: () => {
      toast.success('Update channel changed');
      queryClient.invalidateQueries({ queryKey: ['updates'] });
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to change channel');
    },
  });

  const applyUpdateMutation = useMutation({
    mutationFn: updatesApi.applyUpdate,
    onSuccess: () => {
      toast.success('Update started');
      startProgressStream();
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to start update');
    },
  });

  const rollbackMutation = useMutation({
    mutationFn: updatesApi.rollback,
    onSuccess: () => {
      toast.success('Rollback initiated');
      queryClient.invalidateQueries({ queryKey: ['updates'] });
    },
    onError: (error: any) => {
      toast.error(error.message || 'Rollback failed');
    },
  });

  const deleteSnapshotMutation = useMutation({
    mutationFn: updatesApi.deleteSnapshot,
    onSuccess: () => {
      toast.success('Snapshot deleted');
      refetchSnapshots();
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to delete snapshot');
    },
  });

  // Start progress streaming
  const startProgressStream = () => {
    if (eventSource) {
      eventSource.close();
    }

    const source = updatesApi.streamProgress((progress) => {
      setUpdateProgress(progress);

      // Check if update is complete
      if (
        progress.state === 'success' ||
        progress.state === 'failed' ||
        progress.state === 'rolled_back'
      ) {
        source.close();
        setEventSource(null);
        queryClient.invalidateQueries({ queryKey: ['updates'] });
        
        if (progress.state === 'success') {
          toast.success('Update completed successfully!');
        } else if (progress.state === 'failed') {
          toast.error('Update failed: ' + (progress.error || 'Unknown error'));
        }
      }
    });

    setEventSource(source);
  };

  // Check for ongoing update on mount
  useEffect(() => {
    updatesApi.getProgress().then((progress: UpdateProgress | null) => {
      if (progress && progress.state !== 'idle') {
        setUpdateProgress(progress);
        if (progress.state !== 'success' && progress.state !== 'failed') {
          startProgressStream();
        }
      }
    });

    return () => {
      if (eventSource) {
        eventSource.close();
      }
    };
  }, []);

  const handleChannelChange = (channel: UpdateChannel) => {
    setSelectedChannel(channel);
    changeChannelMutation.mutate({ channel });
  };

  const handleApplyUpdate = () => {
    if (updateCheck?.latest_version) {
      applyUpdateMutation.mutate({
        version: updateCheck.latest_version.version,
      });
    }
  };

  const handleRollback = (snapshotId: string) => {
    if (confirm('Are you sure you want to rollback to this snapshot? This will restart the system.')) {
      rollbackMutation.mutate({ snapshot_id: snapshotId });
    }
  };

  const handleDeleteSnapshot = (snapshotId: string) => {
    if (confirm('Are you sure you want to delete this snapshot? This action cannot be undone.')) {
      deleteSnapshotMutation.mutate(snapshotId);
    }
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">System Updates</h1>
          <p className="text-gray-400 mt-1">
            Manage system updates and snapshots
          </p>
        </div>
        <button
          onClick={() => checkForUpdates()}
          disabled={isChecking || updateProgress?.state !== 'idle'}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {isChecking ? (
            <WorkingIcon className="w-4 h-4 animate-spin" />
          ) : (
            <RefreshIcon className="w-4 h-4" />
          )}
          Check for Updates
        </button>
      </div>

      {/* Current Version Card */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-lg font-semibold text-white mb-4">Current Version</h2>
        {version && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="flex items-start gap-3">
              <ServerIcon className="w-5 h-5 text-gray-400 mt-1" />
              <div>
                <p className="text-sm text-gray-400">OS Version</p>
                <p className="text-white font-medium">{version.os_version}</p>
                <p className="text-xs text-gray-500 mt-1">{version.kernel}</p>
              </div>
            </div>
            
            <div className="flex items-start gap-3">
              <AppIcon className="w-5 h-5 text-gray-400 mt-1" />
              <div>
                <p className="text-sm text-gray-400">Components</p>
                <p className="text-white font-medium">{formatVersion(version)}</p>
                {version.build_date && (
                  <p className="text-xs text-gray-500 mt-1">
                    Built {format(new Date(version.build_date), 'PP')}
                  </p>
                )}
              </div>
            </div>
            
            <div className="flex items-start gap-3">
              <ChannelIcon className="w-5 h-5 text-gray-400 mt-1" />
              <div>
                <p className="text-sm text-gray-400">Update Channel</p>
                <div className="flex items-center gap-2 mt-1">
                  <select
                    value={selectedChannel}
                    onChange={(e) => handleChannelChange(e.target.value as UpdateChannel)}
                    className="bg-gray-700 text-white rounded px-2 py-1 text-sm"
                    disabled={updateProgress?.state !== 'idle'}
                  >
                    <option value="stable">Stable</option>
                    <option value="beta">Beta</option>
                  </select>
                  {selectedChannel === 'beta' && (
                    <span className="text-xs text-yellow-500">⚠️ May be unstable</span>
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
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-gradient-to-r from-blue-900 to-blue-800 rounded-lg p-6"
        >
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center gap-2 mb-3">
                <DownloadIcon className="w-5 h-5 text-blue-300" />
                <h2 className="text-lg font-semibold text-white">Update Available</h2>
                {updateCheck.latest_version.critical && (
                  <span className="px-2 py-1 bg-red-600 text-white text-xs rounded">
                    Critical
                  </span>
                )}
              </div>
              
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <p className="text-sm text-blue-200">Version</p>
                  <p className="text-white font-medium">{updateCheck.latest_version.version}</p>
                </div>
                <div>
                  <p className="text-sm text-blue-200">Release Date</p>
                  <p className="text-white font-medium">
                    {format(new Date(updateCheck.latest_version.release_date), 'PP')}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-blue-200">Size</p>
                  <p className="text-white font-medium">
                    {formatBytes(updateCheck.latest_version.size)}
                  </p>
                </div>
                <div>
                  <p className="text-sm text-blue-200">Packages</p>
                  <p className="text-white font-medium">
                    {updateCheck.latest_version.packages.length} packages
                  </p>
                </div>
              </div>
              
              {updateCheck.latest_version.requires_reboot && (
                <p className="text-yellow-300 text-sm mb-4">
                  ⚠️ This update requires a system reboot
                </p>
              )}
            </div>
            
            <button
              onClick={handleApplyUpdate}
              disabled={updateProgress?.state !== 'idle' || applyUpdateMutation.isPending}
              className="px-6 py-3 bg-white text-blue-900 rounded-lg font-medium hover:bg-gray-100 disabled:opacity-50"
            >
              {applyUpdateMutation.isPending ? (
                <WorkingIcon className="w-5 h-5 animate-spin" />
              ) : (
                'Apply Update'
              )}
            </button>
          </div>
          
          {updateCheck.latest_version.changelog_url && (
            <a
              href={updateCheck.latest_version.changelog_url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-300 hover:text-blue-200 text-sm underline"
            >
              View Changelog →
            </a>
          )}
        </motion.div>
      )}

      {/* Update Progress */}
      {updateProgress && updateProgress.state !== 'idle' && (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-gray-800 rounded-lg p-6"
        >
          <h2 className="text-lg font-semibold text-white mb-4">Update Progress</h2>
          
          <div className="space-y-4">
            {/* Progress Bar */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className={`text-sm ${getUpdateStateColor(updateProgress.state)}`}>
                  {updateProgress.message}
                </span>
                <span className="text-sm text-gray-400">{updateProgress.progress}%</span>
              </div>
              <div className="w-full bg-gray-700 rounded-full h-2">
                <motion.div
                  className="bg-blue-600 h-2 rounded-full"
                  initial={{ width: 0 }}
                  animate={{ width: `${updateProgress.progress}%` }}
                  transition={{ duration: 0.5 }}
                />
              </div>
            </div>
            
            {/* Phase Indicators */}
            <div className="flex items-center justify-between text-xs">
              {['preflight', 'snapshot', 'download', 'install', 'postflight', 'cleanup'].map((phase) => (
                <div
                  key={phase}
                  className={`flex flex-col items-center ${
                    updateProgress.phase === phase ? 'text-blue-400' : 'text-gray-500'
                  }`}
                >
                  <div
                    className={`w-8 h-8 rounded-full border-2 flex items-center justify-center ${
                      updateProgress.phase === phase
                        ? 'border-blue-400 bg-blue-400/20'
                        : 'border-gray-600'
                    }`}
                  >
                    {updateProgress.progress >= (
                      phase === 'preflight' ? 20 :
                      phase === 'snapshot' ? 40 :
                      phase === 'download' ? 60 :
                      phase === 'install' ? 80 :
                      phase === 'postflight' ? 95 : 100
                    ) ? (
                      <OkIcon className="w-4 h-4" />
                    ) : (
                      <div className="w-2 h-2 bg-current rounded-full" />
                    )}
                  </div>
                  <span className="mt-1 capitalize">{phase}</span>
                </div>
              ))}
            </div>
            
            {/* Logs */}
            {updateProgress.logs.length > 0 && (
              <div className="bg-gray-900 rounded p-3 max-h-40 overflow-y-auto">
                {updateProgress.logs.slice(-5).map((log, i) => (
                  <div key={i} className="flex items-start gap-2 text-xs">
                    <span className="text-gray-500">
                      {format(new Date(log.timestamp), 'HH:mm:ss')}
                    </span>
                    <span className={
                      log.level === 'error' ? 'text-red-400' :
                      log.level === 'warn' ? 'text-yellow-400' :
                      'text-gray-300'
                    }>
                      {log.message}
                    </span>
                  </div>
                ))}
              </div>
            )}
            
            {/* ETA */}
            {updateProgress.estimated_time_remaining && (
              <div className="flex items-center gap-2 text-sm text-gray-400">
                <TimeIcon className="w-4 h-4" />
                <span>Estimated time remaining: {formatDuration(updateProgress.estimated_time_remaining)}</span>
              </div>
            )}
          </div>
        </motion.div>
      )}

      {/* Snapshots Section */}
      <div className="bg-gray-800 rounded-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">System Snapshots</h2>
          <button
            onClick={() => setShowSnapshots(!showSnapshots)}
            className="text-blue-400 hover:text-blue-300 text-sm"
          >
            {showSnapshots ? 'Hide' : 'Show'} Snapshots
          </button>
        </div>
        
        {showSnapshots && snapshots && (
          <div className="space-y-3">
            {snapshots.snapshots.length === 0 ? (
              <p className="text-gray-400 text-center py-8">No snapshots available</p>
            ) : (
              snapshots.snapshots.map((snapshot: UpdateSnapshot) => (
                <div
                  key={snapshot.id}
                  className="bg-gray-700 rounded-lg p-4 flex items-center justify-between"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <SnapshotIcon className="w-5 h-5 text-gray-400" />
                      <div>
                        <p className="text-white font-medium">{snapshot.id}</p>
                        <p className="text-sm text-gray-400">
                          {format(new Date(snapshot.created_at), 'PPp')} • {formatBytes(snapshot.size)}
                        </p>
                        {snapshot.description && (
                          <p className="text-sm text-gray-500 mt-1">{snapshot.description}</p>
                        )}
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-2">
                    {snapshot.can_rollback && (
                      <button
                        onClick={() => handleRollback(snapshot.id)}
                        className="p-2 text-yellow-400 hover:bg-gray-600 rounded"
                        title="Rollback to this snapshot"
                      >
                        <RollBackIcon className="w-4 h-4" />
                      </button>
                    )}
                    <button
                      onClick={() => handleDeleteSnapshot(snapshot.id)}
                      className="p-2 text-red-400 hover:bg-gray-600 rounded"
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
  );
}
