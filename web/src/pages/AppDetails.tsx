import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { 
  Package,
  Play,
  Square,
  RotateCw,
  ExternalLink,
  Download,
  Trash2,
  ChevronLeft,
  Shield,
  Activity,
  Terminal,
  Settings,
  History,
  AlertCircle,
  CheckCircle,
  XCircle,
  Loader2,
  Copy,
  Pause,
  PlayCircle,
  ArrowUp,
  Clock,
  HardDrive
} from 'lucide-react';
import { appsApi } from '../api/apps';
import type { AppSnapshot, AppEvent, ContainerHealth, PortMapping } from '../api/apps.types';
import { cn } from '../lib/utils';
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
        return 'text-green-500';
      case 'stopped':
        return 'text-gray-500';
      case 'error':
        return 'text-red-500';
      case 'starting':
      case 'stopping':
      case 'upgrading':
        return 'text-yellow-500';
      default:
        return 'text-gray-400';
    }
  };

  const getHealthColor = (status?: string) => {
    switch (status) {
      case 'healthy':
        return 'text-green-500';
      case 'unhealthy':
        return 'text-red-500';
      default:
        return 'text-gray-400';
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
          <Loader2 className="w-8 h-8 animate-spin text-gray-400" />
        </div>
      </div>
    );
  }

  if (!app) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-12">
          <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <p className="text-lg">App not found</p>
          <button
            onClick={() => navigate('/apps')}
            className="btn btn-primary mt-4"
          >
            Back to Apps
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="bg-gray-800 rounded-lg p-6">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <button
              onClick={() => navigate('/apps')}
              className="btn btn-secondary btn-sm mt-1"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <div className="w-16 h-16 bg-gray-700 rounded-lg flex items-center justify-center">
              <Package className="w-8 h-8 text-gray-400" />
            </div>
            <div>
              <h1 className="text-2xl font-bold">{app.name}</h1>
              <div className="flex items-center gap-4 mt-2">
                <span className="text-sm text-gray-400">v{app.version}</span>
                <div className={cn('flex items-center gap-1', getStatusColor(app.status))}>
                  {app.status === 'running' && <CheckCircle className="w-4 h-4" />}
                  {app.status === 'stopped' && <XCircle className="w-4 h-4" />}
                  {app.status === 'error' && <AlertCircle className="w-4 h-4" />}
                  {(app.status === 'starting' || app.status === 'stopping') && 
                    <Loader2 className="w-4 h-4 animate-spin" />}
                  <span className="capitalize">{app.status}</span>
                </div>
                <div className={cn('flex items-center gap-1', getHealthColor(app.health?.status))}>
                  <Shield className="w-4 h-4" />
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
                      <ExternalLink className="w-4 h-4 mr-2" />
                      Open App
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
                <Play className="w-4 h-4 mr-2" />
                Start
              </button>
            ) : app.status === 'running' ? (
              <>
                <button
                  onClick={() => restartMutation.mutate()}
                  disabled={restartMutation.isPending}
                  className="btn btn-secondary"
                >
                  <RotateCw className="w-4 h-4 mr-2" />
                  Restart
                </button>
                <button
                  onClick={() => stopMutation.mutate()}
                  disabled={stopMutation.isPending}
                  className="btn btn-secondary"
                >
                  <Square className="w-4 h-4 mr-2" />
                  Stop
                </button>
              </>
            ) : null}
            <button
              onClick={() => navigate(`/apps/install/${app.id}`)}
              className="btn btn-secondary"
            >
              <ArrowUp className="w-4 h-4 mr-2" />
              Upgrade
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 p-1 bg-gray-800 rounded-lg w-fit">
        {[
          { key: 'health', label: 'Health', icon: Activity },
          { key: 'logs', label: 'Logs', icon: Terminal },
          { key: 'config', label: 'Config', icon: Settings },
          { key: 'snapshots', label: 'Snapshots', icon: History },
        ].map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key as any)}
            className={cn(
              'px-4 py-2 rounded-md transition-colors flex items-center gap-2',
              activeTab === tab.key
                ? 'bg-gray-700 text-white'
                : 'text-gray-400 hover:text-white'
            )}
          >
            <tab.icon className="w-4 h-4" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="bg-gray-800 rounded-lg p-6">
        {activeTab === 'health' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold">Health Status</h2>
              <button
                onClick={() => healthCheckMutation.mutate()}
                disabled={healthCheckMutation.isPending}
                className="btn btn-secondary btn-sm"
              >
                <RotateCw className={cn(
                  "w-4 h-4 mr-2",
                  healthCheckMutation.isPending && "animate-spin"
                )} />
                Check Now
              </button>
            </div>
            
            {/* Overall Health */}
            <div className="bg-gray-700 rounded-lg p-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-400">Overall Health</p>
                  <p className={cn('text-2xl font-semibold', getHealthColor(app.health?.status))}>
                    {app.health?.status || 'Unknown'}
                  </p>
                </div>
                {app.health?.checked_at && (
                  <p className="text-sm text-gray-400">
                    Last checked: {formatDistanceToNow(new Date(app.health.checked_at), { addSuffix: true })}
                  </p>
                )}
              </div>
              {app.health?.message && (
                <p className="text-sm text-gray-300 mt-2">{app.health.message}</p>
              )}
            </div>
            
            {/* Container Health */}
            {app.health?.containers && app.health.containers.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Containers</h3>
                <div className="space-y-2">
                  {app.health.containers.map((container: ContainerHealth, idx: number) => (
                    <div key={idx} className="bg-gray-700 rounded-lg p-3">
                      <div className="flex items-center justify-between">
                        <span className="font-medium">{container.name}</span>
                        <div className="flex items-center gap-4">
                          <span className={cn(
                            'text-sm',
                            container.status === 'running' ? 'text-green-500' : 'text-gray-400'
                          )}>
                            {container.status}
                          </span>
                          {container.health && (
                            <span className={cn(
                              'text-sm',
                              container.health === 'healthy' ? 'text-green-500' : 'text-red-500'
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
                <h3 className="font-medium mb-3">Recent Events</h3>
                <div className="space-y-2">
                  {eventsData.events.slice(0, 5).map((event: AppEvent) => (
                    <div key={event.id} className="bg-gray-700 rounded-lg p-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Clock className="w-4 h-4 text-gray-400" />
                          <span className="text-sm">{event.type}</span>
                        </div>
                        <span className="text-xs text-gray-400">
                          {formatDistanceToNow(new Date(event.timestamp), { addSuffix: true })}
                        </span>
                      </div>
                      {event.details && (
                        <p className="text-xs text-gray-400 mt-1">
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
              <h2 className="text-xl font-semibold">Logs</h2>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setLogsPaused(!logsPaused)}
                  className="btn btn-secondary btn-sm"
                >
                  {logsPaused ? (
                    <>
                      <PlayCircle className="w-4 h-4 mr-2" />
                      Resume
                    </>
                  ) : (
                    <>
                      <Pause className="w-4 h-4 mr-2" />
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
                  <Download className="w-4 h-4 mr-2" />
                  Download
                </button>
              </div>
            </div>
            
            <div className="bg-gray-900 rounded-lg p-4 font-mono text-sm h-96 overflow-y-auto">
              {logs.length === 0 ? (
                <p className="text-gray-500">No logs available</p>
              ) : (
                <>
                  {logs.map((line, idx) => (
                    <div key={idx} className="hover:bg-gray-800 px-2 py-0.5">
                      <span className="text-gray-500 mr-4">{idx + 1}</span>
                      <span className="text-gray-300">{line}</span>
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
            <h2 className="text-xl font-semibold">Configuration</h2>
            
            {/* Environment Variables */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <h3 className="font-medium">Environment Variables</h3>
                <button
                  onClick={() => copyToClipboard(JSON.stringify(app.params, null, 2))}
                  className="btn btn-secondary btn-sm"
                >
                  <Copy className="w-4 h-4 mr-2" />
                  Copy
                </button>
              </div>
              <div className="bg-gray-900 rounded-lg p-4 font-mono text-sm">
                <pre>{JSON.stringify(app.params, null, 2)}</pre>
              </div>
            </div>
            
            {/* Port Mappings */}
            {app.ports && app.ports.length > 0 && (
              <div>
                <h3 className="font-medium mb-3">Port Mappings</h3>
                <div className="space-y-2">
                  {app.ports.map((port: PortMapping, idx: number) => (
                    <div key={idx} className="bg-gray-700 rounded-lg p-3">
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
              <h2 className="text-xl font-semibold">Snapshots</h2>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => {
                  // In production, this would trigger a manual snapshot
                  toast.info('Manual snapshots coming soon');
                }}
              >
                <HardDrive className="w-4 h-4 mr-2" />
                Create Snapshot
              </button>
            </div>
            
            {app.snapshots && app.snapshots.length > 0 ? (
              <div className="space-y-2">
                {app.snapshots.map((snapshot: AppSnapshot) => (
                  <div key={snapshot.id} className="bg-gray-700 rounded-lg p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <p className="font-medium">{snapshot.name}</p>
                        <p className="text-sm text-gray-400">
                          {formatDistanceToNow(new Date(snapshot.timestamp), { addSuffix: true })}
                        </p>
                        <p className="text-xs text-gray-500">
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
                            Confirm Rollback
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
                          <History className="w-4 h-4 mr-2" />
                          Rollback
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-8">
                <HardDrive className="w-12 h-12 text-gray-600 mx-auto mb-4" />
                <p className="text-gray-400">No snapshots available</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Danger Zone */}
      <div className="bg-red-900/20 border border-red-800 rounded-lg p-6">
        <h2 className="text-xl font-semibold text-red-400 mb-4">Danger Zone</h2>
        {!showDeleteConfirm ? (
          <button
            onClick={() => setShowDeleteConfirm(true)}
            className="btn btn-danger"
          >
            <Trash2 className="w-4 h-4 mr-2" />
            Delete App
          </button>
        ) : (
          <div className="space-y-4">
            <p className="text-sm text-gray-300">
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
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    Deleting...
                  </>
                ) : (
                  <>
                    <Trash2 className="w-4 h-4 mr-2" />
                    Confirm Delete
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
