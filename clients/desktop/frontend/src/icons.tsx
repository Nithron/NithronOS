/**
 * Every icon of NithronSync, by what it means, drawn from the shared pixel
 * set (@nithron/icons, copied whole into src/vendor). Nothing else draws an
 * icon. A pixel icon is ink only (currentColor), drawn on 16 by 16 pixels
 * and shown at exactly 16 or 32 px, never scaled to anything else, never
 * spinning.
 */
import { icons as pixel, type IconName } from './vendor/nithron-icons/index.js';

interface IconProps {
  size?: 16 | 32;
  className?: string;
  /** A lamp: dims and comes back in steps (something running). */
  blink?: boolean;
}

function pixelIcon(name: IconName) {
  return function Icon({ size = 16, className = '', blink = false }: IconProps) {
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 16 16"
        fill="currentColor"
        shapeRendering="crispEdges"
        focusable="false"
        aria-hidden
        className={`shrink-0 ${blink ? 'pixel-blink' : ''} ${className}`.trim()}
      >
        <path d={pixel[name].path} />
      </svg>
    );
  };
}

export const SyncIcon = pixelIcon('sync');
export const SettingsIcon = pixelIcon('settings');
export const FolderIcon = pixelIcon('folder');
export const PauseIcon = pixelIcon('pause');
export const ResumeIcon = pixelIcon('resume');
export const RefreshIcon = pixelIcon('refresh');
export const OpenExternalIcon = pixelIcon('external-link');
export const UploadIcon = pixelIcon('upload');
export const DownloadIcon = pixelIcon('download');
export const CloseIcon = pixelIcon('close');
export const BackIcon = pixelIcon('back');
export const OkIcon = pixelIcon('ok');
export const WarningIcon = pixelIcon('warning');
export const FailedIcon = pixelIcon('failed');
export const RunningIcon = pixelIcon('running');
export const OffIcon = pixelIcon('off');

/** The state of a thing, as the set's state icon beside its word. */
export type MarkState = 'ok' | 'warning' | 'failed' | 'running' | 'off';

const markIcon = { ok: OkIcon, warning: WarningIcon, failed: FailedIcon, running: RunningIcon, off: OffIcon };
const markTone = {
  ok: 'text-status-ok',
  warning: 'text-status-warning',
  failed: 'text-status-failed',
  running: 'text-ink',
  off: 'text-ink-2',
};

export function StatusMark({ state, children, className = '' }: { state: MarkState; children: React.ReactNode; className?: string }) {
  const Icon = markIcon[state];
  return (
    <span className={`inline-flex items-center gap-2 ${className}`.trim()}>
      <Icon className={markTone[state]} blink={state === 'running'} />
      <span>{children}</span>
    </span>
  );
}
