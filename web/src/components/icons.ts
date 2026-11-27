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
  CheckCircle,
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
  Info,
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
  XCircle,
  Zap,
} from 'lucide-react'

export type IconComponent = LucideIcon

/**
 * A square drawn on whole pixels of a 16 px slot, for the marks that are
 * already pixel shapes. `blink` makes it the lamp of something running
 * (styles.css stops it under reduced motion). A caller's animate-spin is
 * dropped: nothing spins.
 */
function pixelSquare(name: string, size: number, blink: boolean): IconComponent {
  const offset = (16 - size) / 2
  const icon = forwardRef<SVGSVGElement, LucideProps>(({ className, color: _c, size: _s, strokeWidth: _w, absoluteStrokeWidth: _a, ...rest }, ref) =>
    createElement(
      'svg',
      {
        ref,
        viewBox: '0 0 16 16',
        shapeRendering: 'crispEdges',
        'aria-hidden': true,
        ...rest,
        className: [className?.replace(/\banimate-spin\b/g, ''), 'fill-current', blink ? 'pixel-blink' : ''].filter(Boolean).join(' '),
      },
      createElement('rect', { x: offset, y: offset, width: size, height: size }),
    ),
  )
  icon.displayName = name
  return icon as unknown as IconComponent
}

/** Working: a square that blinks, never a spinner. */
export const WorkingIcon: IconComponent = pixelSquare('WorkingIcon', 8, true)
/** A bullet or a chosen item: a small square, never a dot. */
export const DotIcon: IconComponent = pixelSquare('DotIcon', 4, false)

export const AccountIcon: IconComponent = UserCog
export const ActivityIcon: IconComponent = Activity
export const AddIcon: IconComponent = Plus
export const AddUserIcon: IconComponent = UserPlus
export const AlertsIcon: IconComponent = Bell
export const AppIcon: IconComponent = Package
export const AppsIcon: IconComponent = Boxes
export const AutomationIcon: IconComponent = Zap
export const BackIcon: IconComponent = ArrowLeft
export const BackupIcon: IconComponent = Archive
export const ChannelIcon: IconComponent = GitBranch
export const ChevronDownIcon: IconComponent = ChevronDown
export const ChevronLeftIcon: IconComponent = ChevronLeft
export const ChevronRightIcon: IconComponent = ChevronRight
export const ChevronUpIcon: IconComponent = ChevronUp
export const ChosenIcon: IconComponent = Check
export const CloseIcon: IconComponent = X
export const CloudIcon: IconComponent = Cloud
export const ConflictsIcon: IconComponent = AlertTriangle
export const ConnectionIcon: IconComponent = Wifi
export const CopyIcon: IconComponent = Copy
export const CpuIcon: IconComponent = Cpu
export const DashboardIcon: IconComponent = LayoutDashboard
export const DatabaseIcon: IconComponent = Database
export const DeleteIcon: IconComponent = Trash2
export const DesktopIcon: IconComponent = Monitor
export const DevelopmentIcon: IconComponent = Code
export const DiskIcon: IconComponent = HardDrive
export const DownloadIcon: IconComponent = Download
export const EditIcon: IconComponent = Edit
export const EmptyIcon: IconComponent = Inbox
export const EncryptionIcon: IconComponent = Shield
export const FallingIcon: IconComponent = TrendingDown
export const FailedIcon: IconComponent = XCircle
export const FileIcon: IconComponent = FileText
export const FileChangedIcon: IconComponent = FileEdit
export const FirewallIcon: IconComponent = Shield
export const FolderIcon: IconComponent = FolderOpen
export const GamingIcon: IconComponent = Gamepad
export const GridViewIcon: IconComponent = Grid3x3
export const HealthIcon: IconComponent = Heart
export const HelpIcon: IconComponent = HelpCircle
export const HideIcon: IconComponent = EyeOff
export const HistoryIcon: IconComponent = History
export const HomeIcon: IconComponent = Home
export const InternetIcon: IconComponent = Globe
export const KeyIcon: IconComponent = Key
export const LaptopIcon: IconComponent = Laptop
export const ListViewIcon: IconComponent = List
export const LiveIcon: IconComponent = Activity
export const LockIcon: IconComponent = Lock
export const MailIcon: IconComponent = Mail
export const MaintenanceIcon: IconComponent = Wrench
export const MediaIcon: IconComponent = Film
export const MemoryIcon: IconComponent = MemoryStick
export const MenuIcon: IconComponent = Menu
export const MonitoringIcon: IconComponent = Activity
export const MoreIcon: IconComponent = MoreVertical
export const NetworkIcon: IconComponent = Network
export const NotFoundIcon: IconComponent = FileX
export const NoticeIcon: IconComponent = Info
export const OffSiteIcon: IconComponent = Globe
export const OfflineIcon: IconComponent = CloudOff
export const OkIcon: IconComponent = CheckCircle
export const OpenExternalIcon: IconComponent = ExternalLink
export const OwnerIcon: IconComponent = Crown
export const PauseIcon: IconComponent = Pause
export const PermissionsIcon: IconComponent = Shield
export const PhoneIcon: IconComponent = Smartphone
export const PoolIcon: IconComponent = Database
export const QuickIcon: IconComponent = Zap
export const RefreshIcon: IconComponent = RefreshCw
export const RestartIcon: IconComponent = RotateCw
export const RisingIcon: IconComponent = TrendingUp
export const RollBackIcon: IconComponent = RotateCcw
export const RouterIcon: IconComponent = Router
export const ScheduleIcon: IconComponent = Calendar
export const SearchIcon: IconComponent = Search
export const SecurityIcon: IconComponent = Shield
export const SecurityOffIcon: IconComponent = ShieldOff
export const SendTestIcon: IconComponent = Send
export const ServerIcon: IconComponent = Server
export const SettingsIcon: IconComponent = Settings
export const ShareIcon: IconComponent = Share2
export const SharingIcon: IconComponent = Users
export const ShowIcon: IconComponent = Eye
export const SignOutIcon: IconComponent = LogOut
export const SmartHomeIcon: IconComponent = Home
export const SnapshotIcon: IconComponent = Camera
export const SortIcon: IconComponent = ChevronsUpDown
export const StartIcon: IconComponent = Play
export const StatisticsIcon: IconComponent = BarChart3
export const StopIcon: IconComponent = Square
export const StorageIcon: IconComponent = HardDrive
export const SyncIcon: IconComponent = Cloud
export const SyncedFolderIcon: IconComponent = FolderSync
export const TemperatureIcon: IconComponent = Thermometer
export const TerminalIcon: IconComponent = Terminal
export const TimeIcon: IconComponent = Clock
export const TurnedOffIcon: IconComponent = ToggleLeft
export const TurnedOnIcon: IconComponent = ToggleRight
export const TwoFactorIcon: IconComponent = Shield
export const UnlockIcon: IconComponent = Unlock
export const UnreachableIcon: IconComponent = ServerCrash
export const UpgradeIcon: IconComponent = ArrowUp
export const UploadIcon: IconComponent = Upload
export const UserIcon: IconComponent = User
export const UsersIcon: IconComponent = Users
export const UtilitiesIcon: IconComponent = Server
export const WarningIcon: IconComponent = AlertTriangle

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
