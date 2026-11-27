// @nithron/icons: generated from src/icons.mjs by scripts/build.mjs. Do not edit.
export declare const GRID: 16;
export declare const SIZES: readonly [16, 32];
export type IconSize = 16 | 32;
export type IconGroup = 'navigation' | 'actions' | 'states' | 'work';
export type IconName =
  | 'dashboard'
  | 'storage'
  | 'disk'
  | 'pool'
  | 'health'
  | 'share'
  | 'folder'
  | 'file'
  | 'sync'
  | 'backup'
  | 'snapshot'
  | 'schedule'
  | 'off-site'
  | 'app'
  | 'monitoring'
  | 'settings'
  | 'user'
  | 'users'
  | 'key'
  | 'lock'
  | 'unlock'
  | 'shield'
  | 'device-phone'
  | 'device-laptop'
  | 'server'
  | 'network'
  | 'chat'
  | 'terminal'
  | 'add'
  | 'remove'
  | 'delete'
  | 'edit'
  | 'copy'
  | 'refresh'
  | 'search'
  | 'upload'
  | 'download'
  | 'send'
  | 'external-link'
  | 'menu'
  | 'more'
  | 'close'
  | 'back'
  | 'up'
  | 'down'
  | 'chevron-left'
  | 'chevron-right'
  | 'chevron-up'
  | 'chevron-down'
  | 'sort'
  | 'check'
  | 'eye'
  | 'eye-off'
  | 'bell'
  | 'ok'
  | 'warning'
  | 'failed'
  | 'running'
  | 'notice'
  | 'off'
  | 'backend'
  | 'architecture'
  | 'data'
  | 'platform'
  | 'security';
export interface PixelIcon {
  readonly group: IconGroup;
  /** What the icon means, in one line. */
  readonly meaning: string;
  /** Who asked for it: nOS, nAI, Niro, sites. */
  readonly products: readonly string[];
  /** The d of its one path, in a viewBox of 0 0 16 16. */
  readonly path: string;
}
export declare const names: readonly IconName[];
export declare const icons: Readonly<Record<IconName, PixelIcon>>;
export default icons;
