/**
 * Every icon of the interface, by what it means in nOS, drawn from the
 * shared pixel set (@nithron/icons, copied whole into src/vendor). Nothing
 * else draws an icon: the lint rule refuses lucide-react, and pages import
 * the meanings below.
 *
 * A pixel icon is ink only (currentColor), drawn on 16 by 16 pixels and
 * shown at exactly 16 px or 32 px: a caller's h-/w- classes only choose
 * between the two (h-8 and larger mean 32), so it is never scaled to
 * anything else or smoothed. Nothing spins: a caller's animate-spin is
 * dropped. A meaning the set has no drawing for yet shows nothing, so its
 * words stand alone (docs/dev/ui-identity-proposal.md lists them for the
 * next round of the set).
 */
import { createElement, forwardRef, type SVGProps } from 'react'
import { icons as pixel, type IconName as PixelName } from '@/vendor/nithron-icons/index.js'

export type IconProps = SVGProps<SVGSVGElement> & {
  /** Accepted for old callers; the size comes from the class (16 or 32). */
  size?: number | string
  strokeWidth?: number | string
}
export type IconComponent = ReturnType<typeof forwardRef<SVGSVGElement, IconProps>>

const SIZE_CLASS = /(?<![\w-])(?:[\w-]+:)?(?:h|w|size)-(?:\d+(?:\.\d+)?|\[[^\]]+\])(?![\w-])/g
const SPIN = /(?<![\w-])animate-spin(?![\w-])/g
const LARGE = /(?<![\w-])(?:h|w|size)-(?:8|9|10|11|12|14|16|20)(?![\w-])/

/** 32 px when the caller asked for a large icon (h-8 and up), else 16. */
function pixelSize(className?: string): 16 | 32 {
  return className && LARGE.test(className) ? 32 : 16
}

function pixelIcon(name: PixelName, displayName: string, blink = false): IconComponent {
  const icon = forwardRef<SVGSVGElement, IconProps>(({ className, size: _s, strokeWidth: _w, ...rest }, ref) => {
    const px = pixelSize(className)
    const cls = [className?.replace(SIZE_CLASS, '').replace(SPIN, ''), 'shrink-0', blink ? 'pixel-blink' : '']
      .filter(Boolean)
      .join(' ')
      .replace(/\s+/g, ' ')
      .trim()
    return createElement(
      'svg',
      {
        ref,
        width: px,
        height: px,
        viewBox: '0 0 16 16',
        fill: 'currentColor',
        shapeRendering: 'crispEdges',
        focusable: 'false',
        'aria-hidden': rest['aria-label'] ? undefined : true,
        ...rest,
        className: cls,
      },
      createElement('path', { d: pixel[name].path }),
    )
  })
  icon.displayName = displayName
  return icon
}

/** A meaning the set has no drawing for yet: nothing, its words stand alone. */
function wordsOnly(displayName: string): IconComponent {
  const icon = forwardRef<SVGSVGElement, IconProps>(() => null)
  icon.displayName = displayName
  return icon
}

export const OkIcon: IconComponent = pixelIcon('ok', 'OkIcon')
export const WarningIcon: IconComponent = pixelIcon('warning', 'WarningIcon')
export const FailedIcon: IconComponent = pixelIcon('failed', 'FailedIcon')
export const NoticeIcon: IconComponent = pixelIcon('notice', 'NoticeIcon')
export const WorkingIcon: IconComponent = pixelIcon('running', 'WorkingIcon', true)
export const DotIcon: IconComponent = pixelIcon('check', 'DotIcon')
export const AccountIcon: IconComponent = pixelIcon('user', 'AccountIcon')
export const ActivityIcon: IconComponent = wordsOnly('ActivityIcon')
export const AddIcon: IconComponent = pixelIcon('add', 'AddIcon')
export const AddUserIcon: IconComponent = pixelIcon('add', 'AddUserIcon')
export const AlertsIcon: IconComponent = pixelIcon('bell', 'AlertsIcon')
export const AppIcon: IconComponent = pixelIcon('app', 'AppIcon')
export const AppsIcon: IconComponent = pixelIcon('app', 'AppsIcon')
export const AutomationIcon: IconComponent = wordsOnly('AutomationIcon')
export const BackIcon: IconComponent = pixelIcon('back', 'BackIcon')
export const BackupIcon: IconComponent = pixelIcon('backup', 'BackupIcon')
export const ChannelIcon: IconComponent = wordsOnly('ChannelIcon')
export const ChevronDownIcon: IconComponent = pixelIcon('chevron-down', 'ChevronDownIcon')
export const ChevronLeftIcon: IconComponent = pixelIcon('chevron-left', 'ChevronLeftIcon')
export const ChevronRightIcon: IconComponent = pixelIcon('chevron-right', 'ChevronRightIcon')
export const ChevronUpIcon: IconComponent = pixelIcon('chevron-up', 'ChevronUpIcon')
export const ChosenIcon: IconComponent = pixelIcon('check', 'ChosenIcon')
export const CloseIcon: IconComponent = pixelIcon('close', 'CloseIcon')
export const CloudIcon: IconComponent = wordsOnly('CloudIcon')
export const ConflictsIcon: IconComponent = wordsOnly('ConflictsIcon')
export const ConnectionIcon: IconComponent = wordsOnly('ConnectionIcon')
export const CopyIcon: IconComponent = pixelIcon('copy', 'CopyIcon')
export const CpuIcon: IconComponent = wordsOnly('CpuIcon')
export const DashboardIcon: IconComponent = pixelIcon('dashboard', 'DashboardIcon')
export const DatabaseIcon: IconComponent = pixelIcon('data', 'DatabaseIcon')
export const DeleteIcon: IconComponent = pixelIcon('delete', 'DeleteIcon')
export const DesktopIcon: IconComponent = wordsOnly('DesktopIcon')
export const DevelopmentIcon: IconComponent = wordsOnly('DevelopmentIcon')
export const DiskIcon: IconComponent = pixelIcon('disk', 'DiskIcon')
export const DownloadIcon: IconComponent = pixelIcon('download', 'DownloadIcon')
export const EditIcon: IconComponent = pixelIcon('edit', 'EditIcon')
export const EmptyIcon: IconComponent = wordsOnly('EmptyIcon')
export const EncryptionIcon: IconComponent = pixelIcon('lock', 'EncryptionIcon')
export const FallingIcon: IconComponent = wordsOnly('FallingIcon')
export const FileIcon: IconComponent = pixelIcon('file', 'FileIcon')
export const FileChangedIcon: IconComponent = wordsOnly('FileChangedIcon')
export const FirewallIcon: IconComponent = pixelIcon('shield', 'FirewallIcon')
export const FolderIcon: IconComponent = pixelIcon('folder', 'FolderIcon')
export const GamingIcon: IconComponent = wordsOnly('GamingIcon')
export const GridViewIcon: IconComponent = wordsOnly('GridViewIcon')
export const HealthIcon: IconComponent = pixelIcon('health', 'HealthIcon')
export const HelpIcon: IconComponent = wordsOnly('HelpIcon')
export const HideIcon: IconComponent = pixelIcon('eye-off', 'HideIcon')
export const HistoryIcon: IconComponent = wordsOnly('HistoryIcon')
export const HomeIcon: IconComponent = wordsOnly('HomeIcon')
export const InternetIcon: IconComponent = wordsOnly('InternetIcon')
export const KeyIcon: IconComponent = pixelIcon('key', 'KeyIcon')
export const LaptopIcon: IconComponent = pixelIcon('device-laptop', 'LaptopIcon')
export const ListViewIcon: IconComponent = wordsOnly('ListViewIcon')
export const LiveIcon: IconComponent = pixelIcon('running', 'LiveIcon')
export const LockIcon: IconComponent = pixelIcon('lock', 'LockIcon')
export const MailIcon: IconComponent = wordsOnly('MailIcon')
export const MaintenanceIcon: IconComponent = wordsOnly('MaintenanceIcon')
export const MediaIcon: IconComponent = wordsOnly('MediaIcon')
export const MemoryIcon: IconComponent = wordsOnly('MemoryIcon')
export const MenuIcon: IconComponent = pixelIcon('menu', 'MenuIcon')
export const MonitoringIcon: IconComponent = pixelIcon('monitoring', 'MonitoringIcon')
export const MoreIcon: IconComponent = pixelIcon('more', 'MoreIcon')
export const NetworkIcon: IconComponent = pixelIcon('network', 'NetworkIcon')
export const NotFoundIcon: IconComponent = wordsOnly('NotFoundIcon')
export const OffSiteIcon: IconComponent = pixelIcon('off-site', 'OffSiteIcon')
export const OfflineIcon: IconComponent = pixelIcon('off', 'OfflineIcon')
export const OpenExternalIcon: IconComponent = pixelIcon('external-link', 'OpenExternalIcon')
export const OwnerIcon: IconComponent = wordsOnly('OwnerIcon')
export const PauseIcon: IconComponent = wordsOnly('PauseIcon')
export const PermissionsIcon: IconComponent = wordsOnly('PermissionsIcon')
export const PhoneIcon: IconComponent = pixelIcon('device-phone', 'PhoneIcon')
export const PoolIcon: IconComponent = pixelIcon('pool', 'PoolIcon')
export const QuickIcon: IconComponent = wordsOnly('QuickIcon')
export const RefreshIcon: IconComponent = pixelIcon('refresh', 'RefreshIcon')
export const RestartIcon: IconComponent = wordsOnly('RestartIcon')
export const RisingIcon: IconComponent = wordsOnly('RisingIcon')
export const RollBackIcon: IconComponent = wordsOnly('RollBackIcon')
export const RouterIcon: IconComponent = wordsOnly('RouterIcon')
export const ScheduleIcon: IconComponent = pixelIcon('schedule', 'ScheduleIcon')
export const SearchIcon: IconComponent = pixelIcon('search', 'SearchIcon')
export const SecurityIcon: IconComponent = pixelIcon('shield', 'SecurityIcon')
export const SecurityOffIcon: IconComponent = wordsOnly('SecurityOffIcon')
export const SendTestIcon: IconComponent = pixelIcon('send', 'SendTestIcon')
export const ServerIcon: IconComponent = pixelIcon('server', 'ServerIcon')
export const SettingsIcon: IconComponent = pixelIcon('settings', 'SettingsIcon')
export const ShareIcon: IconComponent = pixelIcon('share', 'ShareIcon')
export const SharingIcon: IconComponent = pixelIcon('users', 'SharingIcon')
export const ShowIcon: IconComponent = pixelIcon('eye', 'ShowIcon')
export const SignOutIcon: IconComponent = wordsOnly('SignOutIcon')
export const SmartHomeIcon: IconComponent = wordsOnly('SmartHomeIcon')
export const SnapshotIcon: IconComponent = pixelIcon('snapshot', 'SnapshotIcon')
export const SortIcon: IconComponent = pixelIcon('sort', 'SortIcon')
export const StartIcon: IconComponent = wordsOnly('StartIcon')
export const StatisticsIcon: IconComponent = wordsOnly('StatisticsIcon')
export const StopIcon: IconComponent = wordsOnly('StopIcon')
export const StorageIcon: IconComponent = pixelIcon('storage', 'StorageIcon')
export const SyncIcon: IconComponent = pixelIcon('sync', 'SyncIcon')
export const SyncedFolderIcon: IconComponent = wordsOnly('SyncedFolderIcon')
export const TemperatureIcon: IconComponent = wordsOnly('TemperatureIcon')
export const TerminalIcon: IconComponent = pixelIcon('terminal', 'TerminalIcon')
export const TimeIcon: IconComponent = wordsOnly('TimeIcon')
export const TurnedOffIcon: IconComponent = wordsOnly('TurnedOffIcon')
export const TurnedOnIcon: IconComponent = wordsOnly('TurnedOnIcon')
export const TwoFactorIcon: IconComponent = pixelIcon('key', 'TwoFactorIcon')
export const UnlockIcon: IconComponent = pixelIcon('unlock', 'UnlockIcon')
export const UnreachableIcon: IconComponent = wordsOnly('UnreachableIcon')
export const UpgradeIcon: IconComponent = pixelIcon('up', 'UpgradeIcon')
export const UploadIcon: IconComponent = pixelIcon('upload', 'UploadIcon')
export const UserIcon: IconComponent = pixelIcon('user', 'UserIcon')
export const UsersIcon: IconComponent = pixelIcon('users', 'UsersIcon')
export const UtilitiesIcon: IconComponent = wordsOnly('UtilitiesIcon')

/** The icons by name, for code that picks one at run time. */
export const icons = {
  'account': AccountIcon,
  'activity': ActivityIcon,
  'add': AddIcon,
  'add-user': AddUserIcon,
  'alerts': AlertsIcon,
  'app': AppIcon,
  'apps': AppsIcon,
  'automation': AutomationIcon,
  'back': BackIcon,
  'backup': BackupIcon,
  'channel': ChannelIcon,
  'chevron-down': ChevronDownIcon,
  'chevron-left': ChevronLeftIcon,
  'chevron-right': ChevronRightIcon,
  'chevron-up': ChevronUpIcon,
  'chosen': ChosenIcon,
  'close': CloseIcon,
  'cloud': CloudIcon,
  'conflicts': ConflictsIcon,
  'connection': ConnectionIcon,
  'copy': CopyIcon,
  'cpu': CpuIcon,
  'dashboard': DashboardIcon,
  'database': DatabaseIcon,
  'delete': DeleteIcon,
  'desktop': DesktopIcon,
  'development': DevelopmentIcon,
  'disk': DiskIcon,
  'dot': DotIcon,
  'download': DownloadIcon,
  'edit': EditIcon,
  'empty': EmptyIcon,
  'encryption': EncryptionIcon,
  'falling': FallingIcon,
  'failed': FailedIcon,
  'file': FileIcon,
  'file-changed': FileChangedIcon,
  'firewall': FirewallIcon,
  'folder': FolderIcon,
  'gaming': GamingIcon,
  'grid-view': GridViewIcon,
  'health': HealthIcon,
  'help': HelpIcon,
  'hide': HideIcon,
  'history': HistoryIcon,
  'home': HomeIcon,
  'internet': InternetIcon,
  'key': KeyIcon,
  'laptop': LaptopIcon,
  'list-view': ListViewIcon,
  'live': LiveIcon,
  'lock': LockIcon,
  'mail': MailIcon,
  'maintenance': MaintenanceIcon,
  'media': MediaIcon,
  'memory': MemoryIcon,
  'menu': MenuIcon,
  'monitoring': MonitoringIcon,
  'more': MoreIcon,
  'network': NetworkIcon,
  'not-found': NotFoundIcon,
  'notice': NoticeIcon,
  'off-site': OffSiteIcon,
  'offline': OfflineIcon,
  'ok': OkIcon,
  'open-external': OpenExternalIcon,
  'owner': OwnerIcon,
  'pause': PauseIcon,
  'permissions': PermissionsIcon,
  'phone': PhoneIcon,
  'pool': PoolIcon,
  'quick': QuickIcon,
  'refresh': RefreshIcon,
  'restart': RestartIcon,
  'rising': RisingIcon,
  'roll-back': RollBackIcon,
  'router': RouterIcon,
  'schedule': ScheduleIcon,
  'search': SearchIcon,
  'security': SecurityIcon,
  'security-off': SecurityOffIcon,
  'send-test': SendTestIcon,
  'server': ServerIcon,
  'settings': SettingsIcon,
  'share': ShareIcon,
  'sharing': SharingIcon,
  'show': ShowIcon,
  'sign-out': SignOutIcon,
  'smart-home': SmartHomeIcon,
  'snapshot': SnapshotIcon,
  'sort': SortIcon,
  'start': StartIcon,
  'statistics': StatisticsIcon,
  'stop': StopIcon,
  'storage': StorageIcon,
  'sync': SyncIcon,
  'synced-folder': SyncedFolderIcon,
  'temperature': TemperatureIcon,
  'terminal': TerminalIcon,
  'time': TimeIcon,
  'turned-off': TurnedOffIcon,
  'turned-on': TurnedOnIcon,
  'two-factor': TwoFactorIcon,
  'unlock': UnlockIcon,
  'unreachable': UnreachableIcon,
  'upgrade': UpgradeIcon,
  'upload': UploadIcon,
  'user': UserIcon,
  'users': UsersIcon,
  'utilities': UtilitiesIcon,
  'warning': WarningIcon,
  'working': WorkingIcon,
} as const satisfies Record<string, IconComponent>

export type IconName = keyof typeof icons
