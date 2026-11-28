import { useState, useEffect, type ReactNode } from 'react';
import {
  BackIcon,
  CloseIcon,
  DownloadIcon,
  FailedIcon,
  FolderIcon,
  OpenExternalIcon,
  PauseIcon,
  RefreshIcon,
  ResumeIcon,
  SettingsIcon,
  StatusMark,
  UploadIcon,
  type MarkState,
} from './icons';

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
  pairing_link: string;
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

/** Wails rejects with the Go error's text, not an Error. */
function errorText(err: unknown, fallback: string): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string' && err) return err;
  return fallback;
}

function App() {
  const [config, setConfig] = useState<Config | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [activity, setActivity] = useState<Activity[]>([]);
  const [view, setView] = useState<View>('main');
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');

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
      setLoadError('');

      if (!cfg.is_configured) {
        setView('setup');
      }
    } catch (err) {
      setLoadError(errorText(err, 'NithronSync could not read its state'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();

    window.runtime.EventsOn('sync:state', (state: string) => {
      setStatus((prev) => (prev ? { ...prev, state } : prev));
    });

    window.runtime.EventsOn('sync:progress', (data: Partial<Status>) => {
      setStatus((prev) => (prev ? { ...prev, ...data } : prev));
    });

    const interval = setInterval(loadData, 5000);

    return () => {
      clearInterval(interval);
      window.runtime.EventsOff('sync:state');
      window.runtime.EventsOff('sync:progress');
    };
  }, []);

  return (
    <div className="min-h-screen bg-page text-ink">
      <TitleBar />
      {loading ? (
        <main className="p-6">
          <p className="pixel-display pixel-cursor text-ink-2">Reading the sync state</p>
        </main>
      ) : !config || !status ? (
        <main className="p-6">
          <p role="alert" className="flex items-start gap-2">
            <FailedIcon className="mt-0.5 text-status-failed" />
            {loadError || 'NithronSync could not read its state'}
          </p>
        </main>
      ) : view === 'setup' ? (
        <SetupView onComplete={() => { setView('main'); loadData(); }} />
      ) : view === 'settings' ? (
        <SettingsView config={config} onBack={() => setView('main')} onUpdate={loadData} />
      ) : (
        <MainView config={config} status={status} activity={activity} onSettings={() => setView('settings')} />
      )}
    </div>
  );
}

/** The window's own title bar: the name in the pixel face, two controls. */
function TitleBar() {
  return (
    <header className="drag-region flex h-10 items-center justify-between border-b border-hairline px-4">
      <span className="pixel-display no-drag">NithronSync</span>
      <div className="no-drag flex items-center gap-1">
        {/* The set has no drawing for minimise: the word stands alone. */}
        <button type="button" onClick={() => window.runtime.WindowMinimise()} className="h-8 rounded-control px-2 text-xs text-ink-2 hover:bg-ink/[.09] hover:text-ink">
          Minimise
        </button>
        <button type="button" onClick={() => window.runtime.WindowClose()} className="icon-btn" aria-label="Close the window">
          <CloseIcon />
        </button>
      </div>
    </header>
  );
}

function FieldError({ id, children }: { id: string; children: ReactNode }) {
  return (
    <p id={id} role="alert" className="mt-2 flex items-start gap-2 text-status-failed-text">
      <FailedIcon className="mt-0.5 text-status-failed" />
      <span>{children}</span>
    </p>
  );
}

function SetupView({ onComplete }: { onComplete: () => void }) {
  const [pairingLink, setPairingLink] = useState('');
  const [syncFolder, setSyncFolder] = useState('');
  const [error, setError] = useState('');
  const [connecting, setConnecting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setConnecting(true);

    try {
      await window.go.app.App.Setup({
        pairing_link: pairingLink.trim(),
        sync_folder: syncFolder,
      });
      onComplete();
    } catch (err) {
      setError(errorText(err, 'The pairing link was not accepted'));
    } finally {
      setConnecting(false);
    }
  };

  const handleSelectFolder = async () => {
    const folder = await window.go.app.App.SelectFolder();
    if (folder) {
      setSyncFolder(folder);
    }
  };

  return (
    <main className="mx-auto max-w-md p-6">
      <h1 className="screen-title">Connect to nOS</h1>
      <p className="mt-2 text-ink-2">Paste the pairing link that nOS shows for a new device.</p>

      <form onSubmit={handleSubmit} className="mt-6 space-y-5">
        <div>
          <label htmlFor="pairing-link" className="mb-2 block font-[560]">
            Pairing link
          </label>
          <textarea
            id="pairing-link"
            value={pairingLink}
            onChange={(e) => setPairingLink(e.target.value)}
            placeholder="nos://sync?server=..."
            rows={3}
            className="field font-mono text-xs"
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'pairing-error' : 'pairing-help'}
            required
          />
          {error ? (
            <FieldError id="pairing-error">{error}</FieldError>
          ) : (
            <p id="pairing-help" className="mt-2 text-xs text-ink-2">
              In nOS: Sync, Devices, then add a device.
            </p>
          )}
        </div>

        <div>
          <label htmlFor="sync-folder" className="mb-2 block font-[560]">
            Folder on this computer
          </label>
          <div className="flex gap-2">
            <input
              id="sync-folder"
              type="text"
              value={syncFolder}
              onChange={(e) => setSyncFolder(e.target.value)}
              placeholder="The default folder"
              className="field flex-1 font-mono"
            />
            <button type="button" onClick={handleSelectFolder} className="btn btn-line">
              Choose
            </button>
          </div>
        </div>

        <button type="submit" disabled={connecting} className="btn btn-primary w-full">
          {connecting ? 'Connecting' : 'Connect'}
        </button>
      </form>
    </main>
  );
}

function syncState(status: Status): { state: MarkState; words: string } {
  if (!status.is_connected) return { state: 'off', words: 'Not connected' };
  switch (status.state) {
    case 'syncing':
      return { state: 'running', words: 'Syncing' };
    case 'paused':
      return { state: 'off', words: 'Paused' };
    case 'error':
      return { state: 'failed', words: 'Sync stopped on an error' };
    default:
      return { state: 'ok', words: 'Up to date' };
  }
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
  const isSyncing = status.state === 'syncing';
  const isPaused = status.state === 'paused';
  const [actionError, setActionError] = useState('');
  const { state, words } = syncState(status);

  const run = async (what: () => Promise<void>, failed: string) => {
    setActionError('');
    try {
      await what();
    } catch (err) {
      setActionError(`${failed}: ${errorText(err, 'no reason given')}`);
    }
  };

  return (
    <main className="p-6">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h1 className="screen-title">
            <StatusMark state={state}>{words}</StatusMark>
          </h1>
          <p className="mt-1 truncate font-mono text-xs text-ink-2">{status.current_file || config.server_url}</p>
        </div>
        <button type="button" onClick={onSettings} className="btn btn-line shrink-0">
          <SettingsIcon />
          Settings
        </button>
      </div>

      <dl className="mt-6 grid grid-cols-2 border-y border-hairline">
        <Figure icon={<UploadIcon />} label="Waiting to upload" value={status.pending_uploads} detail={`${formatBytes(status.uploaded_bytes)} uploaded`} />
        <Figure icon={<DownloadIcon />} label="Waiting to download" value={status.pending_downloads} detail={`${formatBytes(status.downloaded_bytes)} downloaded`} divider />
      </dl>

      <div className="mt-6 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => run(() => window.go.app.App.SyncNow(), 'Could not start a sync')}
          disabled={isSyncing || isPaused}
          className="btn btn-primary"
        >
          <RefreshIcon />
          Sync now
        </button>
        <button
          type="button"
          onClick={() =>
            run(
              () => (isPaused ? window.go.app.App.ResumeSync() : window.go.app.App.PauseSync()),
              isPaused ? 'Could not resume' : 'Could not pause',
            )
          }
          className="btn btn-line"
        >
          {isPaused ? <ResumeIcon /> : <PauseIcon />}
          {isPaused ? 'Resume' : 'Pause'}
        </button>
        <button type="button" onClick={() => run(() => window.go.app.App.OpenSyncFolder(), 'Could not open the folder')} className="btn btn-line">
          <FolderIcon />
          Open folder
        </button>
      </div>
      {actionError && (
        <p role="alert" className="mt-3 flex items-start gap-2">
          <FailedIcon className="mt-0.5 text-status-failed" />
          {actionError}
        </p>
      )}

      <section className="mt-8" aria-labelledby="activity-title">
        <h2 id="activity-title" className="label mb-2">
          Recent activity
        </h2>
        {activity.length === 0 ? (
          <p className="border-y border-hairline py-4 pixel-display text-ink-2">Nothing has synced yet</p>
        ) : (
          <ul className="max-h-64 divide-y divide-hairline overflow-y-auto border-y border-hairline">
            {activity.map((item) => (
              <li key={item.id} className="py-3">
                <div className="flex items-center justify-between gap-3">
                  <StatusMark state={item.status === 'success' ? 'ok' : 'failed'}>{actionWord(item)}</StatusMark>
                  <span className="shrink-0 font-mono text-xs text-ink-2 tabular-nums">
                    {item.bytes_transferred > 0 && `${formatBytes(item.bytes_transferred)}, `}
                    <time dateTime={item.created_at}>{formatTime(item.created_at)}</time>
                  </span>
                </div>
                <p className="mt-1 truncate pl-6 font-mono text-xs" title={item.path}>
                  {item.path}
                </p>
                {item.status !== 'success' && item.message && <p className="pl-6 text-xs text-ink-2">{item.message}</p>}
              </li>
            ))}
          </ul>
        )}
      </section>

      <footer className="mt-6 border-t border-hairline pt-4">
        <button
          type="button"
          onClick={() => run(() => window.go.app.App.OpenWebUI(), 'Could not open nOS')}
          className="inline-flex items-center gap-2 underline decoration-ink-2 underline-offset-4 hover:decoration-ink"
        >
          Open nOS
          <OpenExternalIcon />
        </button>
      </footer>
    </main>
  );
}

function actionWord(item: Activity): string {
  const failed = item.status !== 'success';
  switch (item.action) {
    case 'upload':
      return failed ? 'Upload failed' : 'Uploaded';
    case 'download':
      return failed ? 'Download failed' : 'Downloaded';
    case 'delete':
      return failed ? 'Delete failed' : 'Deleted';
    default:
      return failed ? 'Failed' : item.action;
  }
}

function Figure({ icon, label, value, detail, divider = false }: { icon: ReactNode; label: string; value: number; detail: string; divider?: boolean }) {
  return (
    <div className={`py-4 ${divider ? 'border-l border-hairline pl-4' : 'pr-4'}`}>
      <dt className="label flex items-center gap-2">
        {icon}
        {label}
      </dt>
      <dd className="mt-2 font-mono text-2xl tabular-nums">{value}</dd>
      <dd className="font-mono text-xs text-ink-2">{detail}</dd>
    </div>
  );
}

function Switch({ id, checked, onChange, label }: { id: string; checked: boolean; onChange: (on: boolean) => void; label: string }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <label htmlFor={id}>{label}</label>
      <span className="flex items-center gap-3">
        <span className="w-7 font-mono text-xs text-ink-2" aria-hidden>
          {checked ? 'On' : 'Off'}
        </span>
        <button id={id} type="button" role="switch" aria-checked={checked} onClick={() => onChange(!checked)} className="switch" />
      </span>
    </div>
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
  const [error, setError] = useState('');

  const handleSave = async () => {
    setSaving(true);
    setError('');
    try {
      await window.go.app.App.UpdateSettings({
        sync_enabled: syncEnabled,
        sync_on_metered: syncOnMetered,
        bandwidth_limit_kbps: bandwidthLimit,
      });
      onUpdate();
      onBack();
    } catch (err) {
      setError(errorText(err, 'The settings could not be saved'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <main className="p-6">
      <div className="flex items-center gap-2">
        <button type="button" onClick={onBack} className="icon-btn" aria-label="Back">
          <BackIcon />
        </button>
        <h1 className="screen-title">Settings</h1>
      </div>

      <section className="mt-6" aria-labelledby="connection-title">
        <h2 id="connection-title" className="label mb-2">
          Connection
        </h2>
        <dl className="divide-y divide-hairline border-y border-hairline">
          <div className="flex justify-between gap-4 py-3">
            <dt className="text-ink-2">Server</dt>
            <dd className="truncate font-mono text-xs">{config.server_url}</dd>
          </div>
          <div className="flex justify-between gap-4 py-3">
            <dt className="text-ink-2">Folder</dt>
            <dd className="truncate font-mono text-xs">{config.sync_folder}</dd>
          </div>
        </dl>
      </section>

      <section className="mt-6" aria-labelledby="sync-title">
        <h2 id="sync-title" className="label mb-2">
          Sync
        </h2>
        <div className="divide-y divide-hairline border-y border-hairline">
          <Switch id="sync-enabled" label="Sync" checked={syncEnabled} onChange={setSyncEnabled} />
          <Switch id="sync-metered" label="Sync on metered connections" checked={syncOnMetered} onChange={setSyncOnMetered} />
          <div className="py-3">
            <label htmlFor="bandwidth" className="mb-2 block">
              Bandwidth limit in KB/s
            </label>
            <input
              id="bandwidth"
              type="number"
              value={bandwidthLimit}
              onChange={(e) => setBandwidthLimit(parseInt(e.target.value) || 0)}
              min="0"
              className="field font-mono"
              aria-describedby="bandwidth-help"
            />
            <p id="bandwidth-help" className="mt-2 text-xs text-ink-2">
              0 means no limit.
            </p>
          </div>
        </div>
      </section>

      {error && (
        <p role="alert" className="mt-4 flex items-start gap-2">
          <FailedIcon className="mt-0.5 text-status-failed" />
          {error}
        </p>
      )}

      <div className="mt-6 flex gap-2">
        <button type="button" onClick={handleSave} disabled={saving} className="btn btn-primary">
          {saving ? 'Saving' : 'Save'}
        </button>
        <button type="button" onClick={onBack} className="btn btn-line">
          Cancel
        </button>
      </div>

      <div className="mt-8 border-t border-hairline pt-4">
        <button type="button" onClick={() => window.go.app.App.Quit()} className="btn btn-line">
          Quit NithronSync
        </button>
      </div>
    </main>
  );
}

function formatBytes(bytes: number): string {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

/** Recent times relative, older ones as a date, in the reader's locale. */
function formatTime(dateStr: string): string {
  const date = new Date(dateStr);
  const diff = Date.now() - date.getTime();
  const rel = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
  if (diff < 60_000) return rel.format(0, 'second');
  if (diff < 3_600_000) return rel.format(-Math.floor(diff / 60_000), 'minute');
  if (diff < 86_400_000) return rel.format(-Math.floor(diff / 3_600_000), 'hour');
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

export default App;
