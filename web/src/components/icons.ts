/**
 * Every icon of the interface, by what it means in nOS. Nothing else imports
 * lucide-react (the lint rule refuses it), so the shared pixel set replaces
 * the icons in this one file. Until then the lucide icons stand behind the
 * names; two names may point at one lucide icon, never one name at two
 * meanings.
 *
 * Sizes: a slot is 16 px (rows, buttons, navigation) or 32 px (a screen
 * title, an empty screen), nothing in between: a pixel icon cannot be scaled
 * by a fraction.
 */
import { createElement, forwardRef } from 'react'
import type { LucideIcon, LucideProps } from 'lucide-react'
import {
  Activity,
  AlertTriangle,
  Archive,
  ArrowLeft,
  ArrowUp,
  BarChart3,
  Bell,
  Boxes,
  Calendar,
  Camera,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  Clock,
  Cloud,
  CloudOff,
  Code,
  Copy,
  Cpu,
  Crown,
  Database,
  Download,
  Edit,
  ExternalLink,
  Eye,
  EyeOff,
  FileEdit,
  FileText,
  FileX,
  Film,
  FolderOpen,
  FolderSync,
  Gamepad,
  GitBranch,
  Globe,
  Grid3x3,
  HardDrive,
  Heart,
  HelpCircle,
  History,
  Home,
  Inbox,
  Key,
  Laptop,
  LayoutDashboard,
  List,
  Lock,
  LogOut,
  Mail,
  MemoryStick,
  Menu,
  Monitor,
  MoreVertical,
  Network,
  Package,
  Pause,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  RotateCw,
  Router,
  Search,
  Send,
  Server,
  ServerCrash,
  Settings,
  Share2,
  Shield,
  ShieldOff,
  Smartphone,
  Square,
  Terminal,
  Thermometer,
  ToggleLeft,
  ToggleRight,
  Trash2,
  TrendingDown,
  TrendingUp,
  Unlock,
  Upload,
  User,
  UserCog,
  UserPlus,
  Users,
  Wifi,
  Wrench,
  X,
  Zap,
} from 'lucide-react'

export type IconComponent = LucideIcon

const SPIN = /(?<![\w-])animate-spin(?![\w-])/g

/** A lucide icon that never spins: a caller's animate-spin is dropped. */
function still(icon: LucideIcon, name: string): IconComponent {
  const wrapped = forwardRef<SVGSVGElement, LucideProps>(({ className, ...rest }, ref) =>
    createElement(icon, { ...rest, ref, className: className?.replace(SPIN, '').trim() }),
  )
  wrapped.displayName = name
  return wrapped as unknown as IconComponent
}

/**
 * A mark drawn on whole pixels: an 8 by 8 shape in the middle of a 16 px
 * slot, one unit a pixel. `blink` makes it the lamp of something running
 * (styles.css stops it under reduced motion). Nothing spins.
 */
function pixelMark(name: string, d: string, blink = false): IconComponent {
  const icon = forwardRef<SVGSVGElement, LucideProps>(
    ({ className, color: _c, size: _s, strokeWidth: _w, absoluteStrokeWidth: _a, ...rest }, ref) =>
      createElement(
        'svg',
        {
          ref,
          viewBox: '-4 -4 16 16',
          shapeRendering: 'crispEdges',
          'aria-hidden': true,
          ...rest,
          className: [className?.replace(SPIN, ''), 'fill-current', blink ? 'pixel-blink' : '']
            .filter(Boolean)
            .join(' '),
        },
        createElement('path', { d, fillRule: 'evenodd' }),
      ),
  )
  icon.displayName = name
  return icon as unknown as IconComponent
}

const SQUARE = 'M0 0h8v8h-8z'
const OPEN_SQUARE = 'M0 0h8v8h-8zM1 1v6h6v-6z'
const SMALL_SQUARE = 'M2 2h4v4h-4z'
const CROSS = Array.from({ length: 8 }, (_, i) => `M${i} ${i}h1v1h-1zM${7 - i} ${i}h1v1h-1z`).join('')

/** States: the marks of components/ui/pixel.tsx, as icons. */
export const OkIcon: IconComponent = pixelMark('OkIcon', SQUARE)
export const WarningIcon: IconComponent = pixelMark('WarningIcon', OPEN_SQUARE)
export const FailedIcon: IconComponent = pixelMark('FailedIcon', CROSS)
export const NoticeIcon: IconComponent = pixelMark('NoticeIcon', SMALL_SQUARE)
/** Working: a square that blinks, never a spinner. */
export const WorkingIcon: IconComponent = pixelMark('WorkingIcon', SQUARE, true)
/** A bullet or a chosen item: a small square, never a dot. */
export const DotIcon: IconComponent = pixelMark('DotIcon', SMALL_SQUARE)

export const AccountIcon: IconComponent = still(UserCog, 'AccountIcon')
export const ActivityIcon: IconComponent = still(Activity, 'ActivityIcon')
export const AddIcon: IconComponent = still(Plus, 'AddIcon')
export const AddUserIcon: IconComponent = still(UserPlus, 'AddUserIcon')
export const AlertsIcon: IconComponent = still(Bell, 'AlertsIcon')
export const AppIcon: IconComponent = still(Package, 'AppIcon')
export const AppsIcon: IconComponent = still(Boxes, 'AppsIcon')
export const AutomationIcon: IconComponent = still(Zap, 'AutomationIcon')
export const BackIcon: IconComponent = still(ArrowLeft, 'BackIcon')
export const BackupIcon: IconComponent = still(Archive, 'BackupIcon')
export const ChannelIcon: IconComponent = still(GitBranch, 'ChannelIcon')
export const ChevronDownIcon: IconComponent = still(ChevronDown, 'ChevronDownIcon')
export const ChevronLeftIcon: IconComponent = still(ChevronLeft, 'ChevronLeftIcon')
export const ChevronRightIcon: IconComponent = still(ChevronRight, 'ChevronRightIcon')
export const ChevronUpIcon: IconComponent = still(ChevronUp, 'ChevronUpIcon')
export const ChosenIcon: IconComponent = still(Check, 'ChosenIcon')
export const CloseIcon: IconComponent = still(X, 'CloseIcon')
export const CloudIcon: IconComponent = still(Cloud, 'CloudIcon')
export const ConflictsIcon: IconComponent = still(AlertTriangle, 'ConflictsIcon')
export const ConnectionIcon: IconComponent = still(Wifi, 'ConnectionIcon')
export const CopyIcon: IconComponent = still(Copy, 'CopyIcon')
export const CpuIcon: IconComponent = still(Cpu, 'CpuIcon')
export const DashboardIcon: IconComponent = still(LayoutDashboard, 'DashboardIcon')
export const DatabaseIcon: IconComponent = still(Database, 'DatabaseIcon')
export const DeleteIcon: IconComponent = still(Trash2, 'DeleteIcon')
export const DesktopIcon: IconComponent = still(Monitor, 'DesktopIcon')
export const DevelopmentIcon: IconComponent = still(Code, 'DevelopmentIcon')
export const DiskIcon: IconComponent = still(HardDrive, 'DiskIcon')
export const DownloadIcon: IconComponent = still(Download, 'DownloadIcon')
export const EditIcon: IconComponent = still(Edit, 'EditIcon')
export const EmptyIcon: IconComponent = still(Inbox, 'EmptyIcon')
export const EncryptionIcon: IconComponent = still(Shield, 'EncryptionIcon')
export const FallingIcon: IconComponent = still(TrendingDown, 'FallingIcon')
export const FileIcon: IconComponent = still(FileText, 'FileIcon')
export const FileChangedIcon: IconComponent = still(FileEdit, 'FileChangedIcon')
export const FirewallIcon: IconComponent = still(Shield, 'FirewallIcon')
export const FolderIcon: IconComponent = still(FolderOpen, 'FolderIcon')
export const GamingIcon: IconComponent = still(Gamepad, 'GamingIcon')
export const GridViewIcon: IconComponent = still(Grid3x3, 'GridViewIcon')
export const HealthIcon: IconComponent = still(Heart, 'HealthIcon')
export const HelpIcon: IconComponent = still(HelpCircle, 'HelpIcon')
export const HideIcon: IconComponent = still(EyeOff, 'HideIcon')
export const HistoryIcon: IconComponent = still(History, 'HistoryIcon')
export const HomeIcon: IconComponent = still(Home, 'HomeIcon')
export const InternetIcon: IconComponent = still(Globe, 'InternetIcon')
export const KeyIcon: IconComponent = still(Key, 'KeyIcon')
export const LaptopIcon: IconComponent = still(Laptop, 'LaptopIcon')
export const ListViewIcon: IconComponent = still(List, 'ListViewIcon')
export const LiveIcon: IconComponent = still(Activity, 'LiveIcon')
export const LockIcon: IconComponent = still(Lock, 'LockIcon')
export const MailIcon: IconComponent = still(Mail, 'MailIcon')
export const MaintenanceIcon: IconComponent = still(Wrench, 'MaintenanceIcon')
export const MediaIcon: IconComponent = still(Film, 'MediaIcon')
export const MemoryIcon: IconComponent = still(MemoryStick, 'MemoryIcon')
export const MenuIcon: IconComponent = still(Menu, 'MenuIcon')
export const MonitoringIcon: IconComponent = still(Activity, 'MonitoringIcon')
export const MoreIcon: IconComponent = still(MoreVertical, 'MoreIcon')
export const NetworkIcon: IconComponent = still(Network, 'NetworkIcon')
export const NotFoundIcon: IconComponent = still(FileX, 'NotFoundIcon')
export const OffSiteIcon: IconComponent = still(Globe, 'OffSiteIcon')
export const OfflineIcon: IconComponent = still(CloudOff, 'OfflineIcon')
export const OpenExternalIcon: IconComponent = still(ExternalLink, 'OpenExternalIcon')
export const OwnerIcon: IconComponent = still(Crown, 'OwnerIcon')
export const PauseIcon: IconComponent = still(Pause, 'PauseIcon')
export const PermissionsIcon: IconComponent = still(Shield, 'PermissionsIcon')
export const PhoneIcon: IconComponent = still(Smartphone, 'PhoneIcon')
export const PoolIcon: IconComponent = still(Database, 'PoolIcon')
export const QuickIcon: IconComponent = still(Zap, 'QuickIcon')
export const RefreshIcon: IconComponent = still(RefreshCw, 'RefreshIcon')
export const RestartIcon: IconComponent = still(RotateCw, 'RestartIcon')
export const RisingIcon: IconComponent = still(TrendingUp, 'RisingIcon')
export const RollBackIcon: IconComponent = still(RotateCcw, 'RollBackIcon')
export const RouterIcon: IconComponent = still(Router, 'RouterIcon')
export const ScheduleIcon: IconComponent = still(Calendar, 'ScheduleIcon')
export const SearchIcon: IconComponent = still(Search, 'SearchIcon')
export const SecurityIcon: IconComponent = still(Shield, 'SecurityIcon')
export const SecurityOffIcon: IconComponent = still(ShieldOff, 'SecurityOffIcon')
export const SendTestIcon: IconComponent = still(Send, 'SendTestIcon')
export const ServerIcon: IconComponent = still(Server, 'ServerIcon')
export const SettingsIcon: IconComponent = still(Settings, 'SettingsIcon')
export const ShareIcon: IconComponent = still(Share2, 'ShareIcon')
export const SharingIcon: IconComponent = still(Users, 'SharingIcon')
export const ShowIcon: IconComponent = still(Eye, 'ShowIcon')
export const SignOutIcon: IconComponent = still(LogOut, 'SignOutIcon')
export const SmartHomeIcon: IconComponent = still(Home, 'SmartHomeIcon')
export const SnapshotIcon: IconComponent = still(Camera, 'SnapshotIcon')
export const SortIcon: IconComponent = still(ChevronsUpDown, 'SortIcon')
export const StartIcon: IconComponent = still(Play, 'StartIcon')
export const StatisticsIcon: IconComponent = still(BarChart3, 'StatisticsIcon')
export const StopIcon: IconComponent = still(Square, 'StopIcon')
export const StorageIcon: IconComponent = still(HardDrive, 'StorageIcon')
export const SyncIcon: IconComponent = still(Cloud, 'SyncIcon')
export const SyncedFolderIcon: IconComponent = still(FolderSync, 'SyncedFolderIcon')
export const TemperatureIcon: IconComponent = still(Thermometer, 'TemperatureIcon')
export const TerminalIcon: IconComponent = still(Terminal, 'TerminalIcon')
export const TimeIcon: IconComponent = still(Clock, 'TimeIcon')
export const TurnedOffIcon: IconComponent = still(ToggleLeft, 'TurnedOffIcon')
export const TurnedOnIcon: IconComponent = still(ToggleRight, 'TurnedOnIcon')
export const TwoFactorIcon: IconComponent = still(Shield, 'TwoFactorIcon')
export const UnlockIcon: IconComponent = still(Unlock, 'UnlockIcon')
export const UnreachableIcon: IconComponent = still(ServerCrash, 'UnreachableIcon')
export const UpgradeIcon: IconComponent = still(ArrowUp, 'UpgradeIcon')
export const UploadIcon: IconComponent = still(Upload, 'UploadIcon')
export const UserIcon: IconComponent = still(User, 'UserIcon')
export const UsersIcon: IconComponent = still(Users, 'UsersIcon')
export const UtilitiesIcon: IconComponent = still(Server, 'UtilitiesIcon')

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
