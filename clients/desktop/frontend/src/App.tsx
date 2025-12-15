import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Cloud,
  CloudOff,
  Settings,
  FolderOpen,
  RefreshCw,
  Pause,
  Play,
  ExternalLink,
  Check,
  AlertCircle,
  Upload,
  Download,
  X,
} from 'lucide-react';

// Wails runtime bindings
declare global {
  interface Window {
    go: {
      app: {
        App: {
          GetConfig: () => Promise<Config>;
          GetStatus: () => Promise<Status>;
          GetStats: () => Promise<Record<string, SyncStats>>;
          GetRecentActivity: (limit: number) => Promise<Activity[]>;
          Setup: (req: SetupRequest) => Promise<void>;
          UpdateSettings: (settings: SettingsUpdate) => Promise<void>;
          PauseSync: () => Promise<void>;
          ResumeSync: () => Promise<void>;
          SyncNow: () => Promise<void>;
          OpenSyncFolder: () => Promise<void>;
          OpenWebUI: () => Promise<void>;
          SelectFolder: () => Promise<string>;
          Quit: () => Promise<void>;
          GetSystemInfo: () => Promise<SystemInfo>;
        };
      };
    };
    runtime: {
      EventsOn: (event: string, callback: (data: any) => void) => void;
      EventsOff: (event: string) => void;
      WindowMinimise: () => void;
      WindowMaximise: () => void;
      WindowUnmaximise: () => void;
      WindowClose: () => void;
      Quit: () => void;
    };
  }
}

interface Config {
  server_url: string;
  sync_folder: string;
  sync_enabled: boolean;
  sync_on_metered: boolean;
  bandwidth_limit_kbps: number;
  poll_interval_secs: number;
  is_configured: boolean;
}

interface Status {
  state: string;
  is_connected: boolean;
  current_file: string;
  uploaded_bytes: number;
  downloaded_bytes: number;
  pending_uploads: number;
  pending_downloads: number;
}

interface SyncStats {
  total_files: number;
  synced_files: number;
  synced_bytes: number;
  pending_upload_files: number;
  pending_download_files: number;
  error_files: number;
  conflict_files: number;
}

interface Activity {
  id: number;
  share_id: string;
  path: string;
  action: string;
  status: string;
  message: string;
  bytes_transferred: number;
  created_at: string;
}

interface SetupRequest {
  server_url: string;
  device_token: string;
  device_name: string;
  sync_folder: string;
}

interface SettingsUpdate {
  sync_enabled?: boolean;
  sync_on_metered?: boolean;
  bandwidth_limit_kbps?: number;
  poll_interval_secs?: number;
  conflict_policy?: string;
}

interface SystemInfo {
  os: string;
  arch: string;
  version: string;
  go_version: string;
  config_dir: string;
  data_dir: string;
  log_dir: string;
}

type View = 'main' | 'setup' | 'settings';

function App() {
  const [config, setConfig] = useState<Config | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [activity, setActivity] = useState<Activity[]>([]);
  const [view, setView] = useState<View>('main');
  const [loading, setLoading] = useState(true);

  const loadData = async () => {
    try {
      const [cfg, sts, act] = await Promise.all([
        window.go.app.App.GetConfig(),
        window.go.app.App.GetStatus(),
        window.go.app.App.GetRecentActivity(10),
      ]);
      setConfig(cfg);
      setStatus(sts);
      setActivity(act || []);

      if (!cfg.is_configured) {
        setView('setup');
      }
    } catch (err) {
      console.error('Failed to load data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();

    // Subscribe to events
    window.runtime.EventsOn('sync:state', (state: string) => {
      setStatus((prev) => (prev ? { ...prev, state } : prev));
    });

    window.runtime.EventsOn('sync:progress', (data: any) => {
      setStatus((prev) => ({
        ...prev!,
        ...data,
      }));
    });

    // Refresh data periodically
    const interval = setInterval(loadData, 5000);

    return () => {
      clearInterval(interval);
      window.runtime.EventsOff('sync:state');
      window.runtime.EventsOff('sync:progress');
    };
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <RefreshCw className="w-8 h-8 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100">
      {/* Title bar */}
      <div className="drag-region h-8 bg-slate-800 flex items-center justify-between px-4">
        <span className="text-white text-sm font-medium no-drag">NithronSync</span>
        <div className="flex items-center gap-1 no-drag">
          <button
            onClick={() => window.runtime.WindowMinimise()}
            className="w-6 h-6 flex items-center justify-center hover:bg-slate-700 rounded"
          >
            <div className="w-3 h-0.5 bg-slate-400" />
          </button>
          <button
            onClick={() => window.runtime.WindowClose()}
            className="w-6 h-6 flex items-center justify-center hover:bg-red-500 rounded"
          >
            <X className="w-3 h-3 text-slate-400" />
          </button>
        </div>
      </div>

      <AnimatePresence mode="wait">
        {view === 'setup' && (
          <SetupView key="setup" onComplete={() => { setView('main'); loadData(); }} />
        )}
        {view === 'settings' && (
          <SettingsView
            key="settings"
            config={config!}
            onBack={() => setView('main')}
            onUpdate={loadData}
          />
        )}
        {view === 'main' && (
          <MainView
            key="main"
            config={config!}
            status={status!}
            activity={activity}
            onSettings={() => setView('settings')}
          />
        )}
      </AnimatePresence>
    </div>
  );
}

function SetupView({ onComplete }: { onComplete: () => void }) {
  const [serverUrl, setServerUrl] = useState('');
  const [deviceToken, setDeviceToken] = useState('');
  const [syncFolder, setSyncFolder] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await window.go.app.App.Setup({
        server_url: serverUrl,
        device_token: deviceToken,
        device_name: '',
        sync_folder: syncFolder,
      });
      onComplete();
    } catch (err: any) {
      setError(err.message || 'Setup failed');
    } finally {
      setLoading(false);
    }
  };

  const handleSelectFolder = async () => {
    const folder = await window.go.app.App.SelectFolder();
    if (folder) {
      setSyncFolder(folder);
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      className="p-8 max-w-md mx-auto"
    >
      <div className="text-center mb-8">
        <Cloud className="w-16 h-16 text-blue-500 mx-auto mb-4" />
        <h1 className="text-2xl font-bold text-slate-800">Welcome to NithronSync</h1>
        <p className="text-slate-600 mt-2">Connect to your NithronOS server to start syncing files.</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-slate-700 mb-1">Server URL</label>
          <input
            type="url"
            value={serverUrl}
            onChange={(e) => setServerUrl(e.target.value)}
            placeholder="https://your-server.local"
            className="w-full px-4 py-2 border border-slate-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-slate-700 mb-1">Device Token</label>
          <input
            type="password"
            value={deviceToken}
            onChange={(e) => setDeviceToken(e.target.value)}
            placeholder="nos_dt_..."
            className="w-full px-4 py-2 border border-slate-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none font-mono text-sm"
            required
          />
          <p className="text-xs text-slate-500 mt-1">
            Get this from NithronOS → Settings → Sync → Devices → Add Device
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium text-slate-700 mb-1">Sync Folder</label>
          <div className="flex gap-2">
            <input
              type="text"
              value={syncFolder}
              onChange={(e) => setSyncFolder(e.target.value)}
              placeholder="Leave empty for default"
              className="flex-1 px-4 py-2 border border-slate-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            />
            <button
              type="button"
              onClick={handleSelectFolder}
              className="px-4 py-2 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors"
            >
              Browse
            </button>
          </div>
        </div>

        {error && (
          <div className="p-3 bg-red-50 border border-red-200 rounded-lg flex items-center gap-2 text-red-700">
            <AlertCircle className="w-5 h-5 flex-shrink-0" />
            <span className="text-sm">{error}</span>
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          className="w-full py-3 bg-blue-500 hover:bg-blue-600 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
        >
          {loading ? (
            <RefreshCw className="w-5 h-5 animate-spin" />
          ) : (
            <>
              <Check className="w-5 h-5" />
              Connect
            </>
          )}
        </button>
      </form>
    </motion.div>
  );
}

function MainView({
  config,
  status,
  activity,
  onSettings,
}: {
  config: Config;
  status: Status;
  activity: Activity[];
  onSettings: () => void;
}) {
  const isConnected = status.is_connected;
  const isSyncing = status.state === 'syncing';
  const isPaused = status.state === 'paused';

  const handlePauseResume = async () => {
    if (isPaused) {
      await window.go.app.App.ResumeSync();
    } else {
      await window.go.app.App.PauseSync();
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="p-6"
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          {isConnected ? (
            <Cloud className={`w-8 h-8 text-green-500 ${isSyncing ? 'animate-pulse-sync' : ''}`} />
          ) : (
            <CloudOff className="w-8 h-8 text-slate-400" />
          )}
          <div>
            <h1 className="text-lg font-semibold text-slate-800">
              {isConnected ? (isSyncing ? 'Syncing...' : isPaused ? 'Paused' : 'Up to date') : 'Disconnected'}
            </h1>
            <p className="text-sm text-slate-500">
              {status.current_file || config.server_url}
            </p>
          </div>
        </div>
        <button
          onClick={onSettings}
          className="p-2 hover:bg-slate-100 rounded-lg transition-colors"
        >
          <Settings className="w-5 h-5 text-slate-600" />
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-4 mb-6">
        <StatCard
          icon={<Upload className="w-5 h-5" />}
          label="Uploads"
          value={status.pending_uploads}
          detail={formatBytes(status.uploaded_bytes)}
          color="blue"
        />
        <StatCard
          icon={<Download className="w-5 h-5" />}
          label="Downloads"
          value={status.pending_downloads}
          detail={formatBytes(status.downloaded_bytes)}
          color="green"
        />
      </div>

      {/* Actions */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => window.go.app.App.OpenSyncFolder()}
          className="flex-1 flex items-center justify-center gap-2 py-2 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors"
        >
          <FolderOpen className="w-4 h-4" />
          Open Folder
        </button>
        <button
          onClick={handlePauseResume}
          className="flex-1 flex items-center justify-center gap-2 py-2 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors"
        >
          {isPaused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
          {isPaused ? 'Resume' : 'Pause'}
        </button>
        <button
          onClick={() => window.go.app.App.SyncNow()}
          disabled={isSyncing || isPaused}
          className="flex-1 flex items-center justify-center gap-2 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${isSyncing ? 'animate-spin' : ''}`} />
          Sync Now
        </button>
      </div>

      {/* Activity */}
      <div>
        <h2 className="text-sm font-medium text-slate-700 mb-2">Recent Activity</h2>
        <div className="bg-white rounded-lg border border-slate-200 divide-y divide-slate-100 max-h-64 overflow-y-auto">
          {activity.length === 0 ? (
            <div className="p-4 text-center text-slate-500 text-sm">No recent activity</div>
          ) : (
            activity.map((item) => (
              <div key={item.id} className="p-3 flex items-center gap-3">
                <div className={`p-1.5 rounded ${item.status === 'success' ? 'bg-green-100' : 'bg-red-100'}`}>
                  {item.action === 'upload' ? (
                    <Upload className={`w-4 h-4 ${item.status === 'success' ? 'text-green-600' : 'text-red-600'}`} />
                  ) : (
                    <Download className={`w-4 h-4 ${item.status === 'success' ? 'text-green-600' : 'text-red-600'}`} />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-slate-800 truncate">{item.path}</p>
                  <p className="text-xs text-slate-500">
                    {formatBytes(item.bytes_transferred)} · {formatTime(item.created_at)}
                  </p>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Footer */}
      <div className="mt-6 pt-4 border-t border-slate-200 flex items-center justify-between">
        <button
          onClick={() => window.go.app.App.OpenWebUI()}
          className="text-sm text-blue-500 hover:text-blue-600 flex items-center gap-1"
        >
          Open NithronOS <ExternalLink className="w-3 h-3" />
        </button>
        <span className="text-xs text-slate-400">NithronSync v1.0.0</span>
      </div>
    </motion.div>
  );
}

function SettingsView({
  config,
  onBack,
  onUpdate,
}: {
  config: Config;
  onBack: () => void;
  onUpdate: () => void;
}) {
  const [syncEnabled, setSyncEnabled] = useState(config.sync_enabled);
  const [syncOnMetered, setSyncOnMetered] = useState(config.sync_on_metered);
  const [bandwidthLimit, setBandwidthLimit] = useState(config.bandwidth_limit_kbps);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await window.go.app.App.UpdateSettings({
        sync_enabled: syncEnabled,
        sync_on_metered: syncOnMetered,
        bandwidth_limit_kbps: bandwidthLimit,
      });
      onUpdate();
      onBack();
    } catch (err) {
      console.error('Failed to save settings:', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <motion.div
      initial={{ opacity: 0, x: 20 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: -20 }}
      className="p-6"
    >
      <div className="flex items-center gap-3 mb-6">
        <button
          onClick={onBack}
          className="p-2 hover:bg-slate-100 rounded-lg transition-colors"
        >
          ←
        </button>
        <h1 className="text-lg font-semibold text-slate-800">Settings</h1>
      </div>

      <div className="space-y-6">
        {/* Connection Info */}
        <div className="bg-white rounded-lg border border-slate-200 p-4">
          <h2 className="text-sm font-medium text-slate-700 mb-3">Connection</h2>
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-slate-500">Server</span>
              <span className="text-slate-800">{config.server_url}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Sync Folder</span>
              <span className="text-slate-800 truncate max-w-48">{config.sync_folder}</span>
            </div>
          </div>
        </div>

        {/* Sync Settings */}
        <div className="bg-white rounded-lg border border-slate-200 p-4">
          <h2 className="text-sm font-medium text-slate-700 mb-3">Sync</h2>
          <div className="space-y-4">
            <label className="flex items-center justify-between">
              <span className="text-sm text-slate-600">Enable sync</span>
              <input
                type="checkbox"
                checked={syncEnabled}
                onChange={(e) => setSyncEnabled(e.target.checked)}
                className="w-5 h-5 rounded text-blue-500"
              />
            </label>
            <label className="flex items-center justify-between">
              <span className="text-sm text-slate-600">Sync on metered connections</span>
              <input
                type="checkbox"
                checked={syncOnMetered}
                onChange={(e) => setSyncOnMetered(e.target.checked)}
                className="w-5 h-5 rounded text-blue-500"
              />
            </label>
            <div>
              <label className="text-sm text-slate-600 block mb-1">
                Bandwidth limit (KB/s, 0 = unlimited)
              </label>
              <input
                type="number"
                value={bandwidthLimit}
                onChange={(e) => setBandwidthLimit(parseInt(e.target.value) || 0)}
                min="0"
                className="w-full px-3 py-2 border border-slate-300 rounded-lg"
              />
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-3">
          <button
            onClick={onBack}
            className="flex-1 py-2 bg-slate-100 hover:bg-slate-200 rounded-lg transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex-1 py-2 bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>

        {/* Quit */}
        <button
          onClick={() => window.go.app.App.Quit()}
          className="w-full py-2 text-red-500 hover:bg-red-50 rounded-lg transition-colors"
        >
          Quit NithronSync
        </button>
      </div>
    </motion.div>
  );
}

function StatCard({
  icon,
  label,
  value,
  detail,
  color,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  detail: string;
  color: 'blue' | 'green';
}) {
  const colors = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
  };

  return (
    <div className="bg-white rounded-lg border border-slate-200 p-4">
      <div className="flex items-center gap-2 mb-2">
        <div className={`p-1.5 rounded ${colors[color]}`}>{icon}</div>
        <span className="text-sm text-slate-600">{label}</span>
      </div>
      <div className="text-2xl font-semibold text-slate-800">{value}</div>
      <div className="text-xs text-slate-500">{detail}</div>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diff = now.getTime() - date.getTime();
  
  if (diff < 60000) return 'Just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return date.toLocaleDateString();
}

export default App;

