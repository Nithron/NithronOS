import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getErrorMessage } from '@/lib/nos-client';
import { AppIcon, ChevronLeftIcon, CopyIcon, DeleteIcon, DownloadIcon, FailedIcon, HistoryIcon, OkIcon, OpenExternalIcon, PauseIcon, RestartIcon, SecurityIcon, StartIcon, StopIcon, StorageIcon, TimeIcon, UpgradeIcon, WorkingIcon } from '@/components/icons'
import { appsApi } from '../api/apps';
import type { AppSnapshot, AppEvent, ContainerHealth, PortMapping } from '../api/apps.types';
import { cn } from '../lib/utils';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { toast } from '@/components/ui/toast';
import { formatDistanceToNow } from 'date-fns';

export function AppDetails() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<'health' | 'logs' | 'config' | 'snapshots'>('health');
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [keepData, setKeepData] = useState(false);
  const [selectedSnapshot, setSelectedSnapshot] = useState<string | null>(null);
  const [logsFollowing] = useState(true);
  const [logsPaused, setLogsPaused] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  // Fetch app details
  const { data: app, isLoading } = useQuery({
    queryKey: ['apps', id],
    queryFn: () => appsApi.getApp(id!),
    refetchInterval: 5000, // Refresh every 5 seconds
  });

  // Fetch events
  const { data: eventsData } = useQuery({
    queryKey: ['apps', id, 'events'],
    queryFn: () => appsApi.getAppEvents(id!, 20),
    refetchInterval: 10000,
  });

  // Mutations
  const startMutation = useMutation({
    mutationFn: () => appsApi.startApp(id!),
    onSuccess: () => {
      toast.success('App started');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
    },
  });

  const snapshotMutation = useMutation({
    mutationFn: () => appsApi.snapshotApp(id!),
    onSuccess: () => {
      toast.success('Snapshot taken');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  });

  const stopMutation = useMutation({
    mutationFn: () => appsApi.stopApp(id!),
    onSuccess: () => {
      toast.success('App stopped');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
    },
  });

  const restartMutation = useMutation({
    mutationFn: () => appsApi.restartApp(id!),
    onSuccess: () => {
      toast.success('App restarted');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => appsApi.deleteApp(id!, keepData),
    onSuccess: () => {
      toast.success('App deleted');
      navigate('/apps');
    },
  });

  const rollbackMutation = useMutation({
    mutationFn: (snapshotTs: string) => appsApi.rollbackApp(id!, snapshotTs),
    onSuccess: () => {
      toast.success('App rolled back successfully');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
      setSelectedSnapshot(null);
    },
  });

  const healthCheckMutation = useMutation({
    mutationFn: () => appsApi.forceHealthCheck(id!),
    onSuccess: () => {
      toast.success('Health check completed');
      queryClient.invalidateQueries({ queryKey: ['apps', id] });
    },
  });

  // WebSocket for logs
  useEffect(() => {
    if (activeTab === 'logs' && logsFollowing && !logsPaused) {
      // Connect WebSocket
      try {
        const ws = appsApi.streamLogs(id!, { follow: true, tail: 100 });
        
        ws.onopen = () => {
          console.log('WebSocket connected');
        };
        
        ws.onmessage = (event) => {
          setLogs(prev => [...prev, event.data].slice(-1000)); // Keep last 1000 lines
        };
        
        ws.onerror = (error) => {
          console.error('WebSocket error:', error);
          // Fallback to HTTP polling
          fetchLogs();
        };
        
        ws.onclose = () => {
          console.log('WebSocket closed');
        };
        
        wsRef.current = ws;
      } catch {
        // Fallback to HTTP
        fetchLogs();
      }
      
      return () => {
        if (wsRef.current) {
          wsRef.current.close();
          wsRef.current = null;
        }
      };
    }
  }, [activeTab, id, logsFollowing, logsPaused]);

  // Fallback HTTP log fetching
  const fetchLogs = async () => {
    try {
      const response = await appsApi.getLogs(id!, { tail: 100 });
      if (typeof response === 'string') {
        setLogs(response.split('\n'));
      }
    } catch (error) {
      console.error('Failed to fetch logs:', error);
    }
  };

  // Auto-scroll logs
  useEffect(() => {
    if (logsFollowing && !logsPaused && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, logsFollowing, logsPaused]);

  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'running':
        return 'text-status-ok-text';
      case 'stopped':
        return 'text-ink-2';
      case 'error':
        return 'text-status-failed-text';
      case 'starting':
      case 'stopping':
      case 'upgrading':
        return 'text-status-warning-text';
      default:
        return 'text-ink-2';
    }
  };

  const getHealthColor = (status?: string) => {
    switch (status) {
      case 'healthy':
        return 'text-status-ok-text';
      case 'unhealthy':
        return 'text-status-failed-text';
      default:
        return 'text-ink-2';
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  if (isLoading) {
    return (
      <div className="container mx-auto py-6">
        <div className="flex justify-center py-12">
          <WorkingIcon className="w-8 h-8 animate-spin text-ink-2" />
        </div>
      </div>
    );
  }

  if (!app) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-12">
          <FailedIcon className="w-12 h-12 text-status-failed-text mx-auto mb-4" />
          <p className="text-lg">App not found</p>
          <button
            onClick={() => navigate('/apps')}
            className="btn btn-primary mt-4"
          >
            Back to apps
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="border border-hairline p-6">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <button
              onClick={() => navigate('/apps')}
              className="btn btn-secondary btn-sm mt-1"
            >
              <ChevronLeftIcon className="w-4 h-4" />
            </button>
            <div className="w-16 h-16 border border-hairline flex items-center justify-center">
              <AppIcon className="w-8 h-8 text-ink-2" />
            </div>
            <div>
              <h1 className="screen-title">{app.name}</h1>
              <div className="flex items-center gap-4 mt-2">
                <span className="text-sm text-ink-2">v{app.version}</span>
                <div className={cn('flex items-center gap-1', getStatusColor(app.status))}>
                  {app.status === 'running' && <OkIcon className="w-4 h-4" />}
                  {app.status === 'stopped' && <FailedIcon className="w-4 h-4" />}
                  {app.status === 'error' && <FailedIcon className="w-4 h-4" />}
                  {(app.status === 'starting' || app.status === 'stopping') && 
                    <WorkingIcon className="w-4 h-4 animate-spin" />}
                  <span className="capitalize">{app.status}</span>
                </div>
                <div className={cn('flex items-center gap-1', getHealthColor(app.health?.status))}>
                  <SecurityIcon className="w-4 h-4" />
                  <span className="capitalize">{app.health?.status || 'unknown'}</span>
                </div>
              </div>
              {app.urls?.length > 0 && (
                <div className="flex items-center gap-2 mt-3">
                  {app.urls.map((url: string, idx: number) => (
                    <a
                      key={idx}
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-secondary btn-sm"
                    >
                      <OpenExternalIcon className="w-4 h-4 mr-2" />
                      Open app
                    </a>
                  ))}
                </div>
              )}
            </div>
          </div>
          
          <div className="flex items-center gap-2">
            {app.status === 'stopped' ? (
              <button
                onClick={() => startMutation.mutate()}
                disabled={startMutation.isPending}
                className="btn btn-primary"
              >
                <StartIcon className="w-4 h-4 mr-2" />
                Start
              </button>
            ) : app.status === 'running' ? (
              <>
                <button
                  onClick={() => restartMutation.mutate()}
                  disabled={restartMutation.isPending}
                  className="btn btn-secondary"
                >
                  <RestartIcon className="w-4 h-4 mr-2" />
                  Restart
                </button>
                <button
                  onClick={() => stopMutation.mutate()}
                  disabled={stopMutation.isPending}
                  className="btn btn-secondary"
                >
                  <StopIcon className="w-4 h-4 mr-2" />
                  Stop
                </button>
              </>
            ) : null}
            <button
              onClick={() => navigate(`/apps/install/${app.id}`)}
              className="btn btn-secondary"
            >
              <UpgradeIcon className="w-4 h-4 mr-2" />
              Upgrade
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as any)}>
        <TabsList>
          <TabsTrigger value="health">Health</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
          <TabsTrigger value="snapshots">Snapshots</TabsTrigger>
        </TabsList>
      </Tabs>

      {/* Tab Content */}
      <div className="border border-hairline p-6">
        {activeTab === 'health' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-[560]">Health status</h2>
              <button
                onClick={() => healthCheckMutation.mutate()}
                disabled={healthCheckMutation.isPending}
                className="btn btn-secondary btn-sm"
              >
                <RestartIcon className={cn(
                  "w-4 h-4 mr-2",
                  healthCheckMutation.isPending && "animate-spin"
                )} />
                Check now
              </button>
            </div>
            
            {/* Overall Health */}
            <div className="border border-hairline p-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-ink-2">Overall health</p>
                  <p className={cn('text-2xl font-[560]', getHealthColor(app.health?.status))}>
                    {app.health?.status || 'Unknown'}
                  </p>
                </div>
                {app.health?.checked_at && (
                  <p className="text-sm text-ink-2">
                    Last checked: {formatDistanceToNow(new Date(app.health.checked_at), { addSuffix: true })}
                  </p>
                )}
              </div>
              {app.health?.message && (
                <p className="text-sm text-ink mt-2">{app.health.message}</p>
              )}
            </div>
            
            {/* Container Health */}
            {app.health?.containers && app.health.containers.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Containers</h3>
                <div className="space-y-2">
                  {app.health.containers.map((container: ContainerHealth, idx: number) => (
                    <div key={idx} className="border border-hairline p-3">
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{container.name}</span>
                        <div className="flex items-center gap-4">
                          <span className={cn(
                            'text-sm',
                            container.status === 'running' ? 'text-status-ok-text' : 'text-ink-2'
                          )}>
                            {container.status}
                          </span>
                          {container.health && (
                            <span className={cn(
                              'text-sm',
                              container.health === 'healthy' ? 'text-status-ok-text' : 'text-status-failed-text'
                            )}>
                              {container.health}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            
            {/* Recent Events */}
            {eventsData?.events && eventsData.events.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Recent events</h3>
                <div className="space-y-2">
                  {eventsData.events.slice(0, 5).map((event: AppEvent) => (
                    <div key={event.id} className="border border-hairline p-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <TimeIcon className="w-4 h-4 text-ink-2" />
                          <span className="text-sm">{event.type}</span>
                        </div>
                        <span className="text-xs text-ink-2">
                          {formatDistanceToNow(new Date(event.timestamp), { addSuffix: true })}
                        </span>
                      </div>
                      {event.details && (
                        <p className="text-xs text-ink-2 mt-1">
                          {JSON.stringify(event.details)}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'logs' && (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-[560]">Logs</h2>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setLogsPaused(!logsPaused)}
                  className="btn btn-secondary btn-sm"
                >
                  {logsPaused ? (
                    <>
                      <StartIcon className="w-4 h-4 mr-2" />
                      Resume
                    </>
                  ) : (
                    <>
                      <PauseIcon className="w-4 h-4 mr-2" />
                      Pause
                    </>
                  )}
                </button>
                <button
                  onClick={() => {
                    const blob = new Blob([logs.join('\n')], { type: 'text/plain' });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = `${app.id}-logs.txt`;
                    a.click();
                    URL.revokeObjectURL(url);
                  }}
                  className="btn btn-secondary btn-sm"
                >
                  <DownloadIcon className="w-4 h-4 mr-2" />
                  Download
                </button>
              </div>
            </div>
            
            <div className="border border-hairline p-4 font-mono text-sm h-96 overflow-y-auto">
              {logs.length === 0 ? (
                <p className="text-ink-2">No logs available</p>
              ) : (
                <>
                  {logs.map((line, idx) => (
                    <div key={idx} className="hover:bg-ink/[.09] px-2 py-0.5">
                      <span className="text-ink-2 mr-4">{idx + 1}</span>
                      <span className="text-ink">{line}</span>
                    </div>
                  ))}
                  <div ref={logsEndRef} />
                </>
              )}
            </div>
          </div>
        )}

        {activeTab === 'config' && (
          <div className="space-y-6">
            <h2 className="text-xl font-[560]">Configuration</h2>
            
            {/* Environment Variables */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <h3 className="font-medium">Environment variables</h3>
                <button
                  onClick={() => copyToClipboard(JSON.stringify(app.params, null, 2))}
                  className="btn btn-secondary btn-sm"
                >
                  <CopyIcon className="w-4 h-4 mr-2" />
                  Copy
                </button>
              </div>
              <div className="border border-hairline p-4 font-mono text-sm">
                <pre>{JSON.stringify(app.params, null, 2)}</pre>
              </div>
            </div>
            
            {/* Port Mappings */}
            {app.ports && app.ports.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Port mappings</h3>
                <div className="space-y-2">
                  {app.ports.map((port: PortMapping, idx: number) => (
                    <div key={idx} className="border border-hairline p-3">
                      <div className="flex items-center justify-between">
                        <span className="text-sm">
                          Container: {port.container}/{port.protocol}
                        </span>
                        <span className="text-sm">
                          Host: {port.host}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'snapshots' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-[560]">Snapshots</h2>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => snapshotMutation.mutate()}
                disabled={snapshotMutation.isPending}
              >
                <StorageIcon className="w-4 h-4 mr-2" />
                Create snapshot
              </button>
            </div>
            
            {app.snapshots && app.snapshots.length > 0 ? (
              <div className="space-y-2">
                {app.snapshots.map((snapshot: AppSnapshot) => (
                  <div key={snapshot.id} className="border border-hairline p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-medium">{snapshot.name}</p>
                        <p className="text-sm text-ink-2">
                          {formatDistanceToNow(new Date(snapshot.timestamp), { addSuffix: true })}
                        </p>
                        <p className="text-xs text-ink-2">
                          Type: {snapshot.type} • ID: {snapshot.id}
                        </p>
                      </div>
                      {selectedSnapshot === snapshot.id ? (
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => rollbackMutation.mutate(snapshot.timestamp)}
                            disabled={rollbackMutation.isPending}
                            className="btn btn-danger btn-sm"
                          >
                            Confirm rollback
                          </button>
                          <button
                            onClick={() => setSelectedSnapshot(null)}
                            className="btn btn-secondary btn-sm"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setSelectedSnapshot(snapshot.id)}
                          className="btn btn-secondary btn-sm"
                        >
                          <HistoryIcon className="w-4 h-4 mr-2" />
                          Rollback
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <StorageIcon className="w-12 h-12 text-ink-2 mx-auto mb-4" />
                <p className="text-ink-2">No snapshots available</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Danger Zone */}
      <div className="border border-status-failed p-6">
        <h2 className="text-xl font-[560] text-status-failed-text mb-4">Danger zone</h2>
        {!showDeleteConfirm ? (
          <button
            onClick={() => setShowDeleteConfirm(true)}
            className="btn btn-danger"
          >
            <DeleteIcon className="w-4 h-4 mr-2" />
            Delete app
          </button>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-ink">
              This will stop and remove the app. You can choose to keep the data.
            </p>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={keepData}
                onChange={(e) => setKeepData(e.target.checked)}
                className="checkbox"
              />
              <span className="text-sm">Keep app data</span>
            </label>
            <div className="flex items-center gap-2">
              <button
                onClick={() => deleteMutation.mutate()}
                disabled={deleteMutation.isPending}
                className="btn btn-danger"
              >
                {deleteMutation.isPending ? (
                  <>
                    <WorkingIcon className="w-4 h-4 mr-2 animate-spin" />
                    Deleting...
                  </>
                ) : (
                  <>
                    <DeleteIcon className="w-4 h-4 mr-2" />
                    Confirm delete
                  </>
                )}
              </button>
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="btn btn-secondary"
              >
                Cancel
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
