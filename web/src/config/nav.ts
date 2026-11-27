import { AppIcon, BackupIcon, ConflictsIcon, DashboardIcon, EncryptionIcon, HealthIcon, HistoryIcon, MonitoringIcon, OffSiteIcon, PhoneIcon, ScheduleIcon, SettingsIcon, ShareIcon, SharingIcon, SnapshotIcon, StorageIcon, SyncIcon, type IconComponent } from '@/components/icons'

export interface NavItem {
  id: string
  label: string
  icon?: IconComponent
  path: string
  children?: NavItem[]
}

export const navItems: NavItem[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: DashboardIcon,
    path: '/',
  },
  {
    id: 'storage',
    label: 'Storage',
    icon: StorageIcon,
    path: '/storage',
  },
  {
    id: 'health',
    label: 'Health',
    icon: HealthIcon,
    path: '/health/system',
  },
  {
    id: 'shares',
    label: 'Shares',
    icon: ShareIcon,
    path: '/shares',
  },
  {
    id: 'sync',
    label: 'Sync',
    icon: SyncIcon,
    path: '/settings/sync',
    children: [
      {
        id: 'sync-settings',
        label: 'Settings',
        icon: SettingsIcon,
        path: '/settings/sync',
      },
      {
        id: 'sync-devices',
        label: 'Devices',
        icon: PhoneIcon,
        path: '/settings/sync/devices',
      },
      {
        id: 'sync-activity',
        label: 'Activity',
        icon: HistoryIcon,
        path: '/settings/sync/activity',
      },
      {
        id: 'sync-conflicts',
        label: 'Conflicts',
        icon: ConflictsIcon,
        path: '/settings/sync/conflicts',
      },
      {
        id: 'sync-sharing',
        label: 'Sharing',
        icon: SharingIcon,
        path: '/settings/sync/sharing',
      },
      {
        id: 'sync-encryption',
        label: 'Encryption',
        icon: EncryptionIcon,
        path: '/settings/sync/encryption',
      },
    ],
  },
  {
    id: 'backup',
    label: 'Backup',
    icon: BackupIcon,
    path: '/backup/snapshots',
    children: [
      {
        id: 'snapshots',
        label: 'Snapshots',
        icon: SnapshotIcon,
        path: '/backup/snapshots',
      },
      {
        id: 'schedules',
        label: 'Schedules',
        icon: ScheduleIcon,
        path: '/backup/schedules',
      },
      {
        id: 'remote',
        label: 'Off-site copies',
        icon: OffSiteIcon,
        path: '/backup/remote',
      },
    ],
  },
  {
    id: 'apps',
    label: 'Apps',
    icon: AppIcon,
    path: '/apps',
  },
  {
    id: 'monitor',
    label: 'Monitoring',
    icon: MonitoringIcon,
    path: '/monitor',
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: SettingsIcon,
    path: '/settings',
    children: [
      {
        id: 'general',
        label: 'General',
        path: '/settings',
      },
      {
        id: 'network',
        label: 'Network & remote',
        path: '/settings/network',
      },
      {
        id: '2fa',
        label: 'Two-factor auth',
        path: '/settings/2fa',
      },
      {
        id: 'users',
        label: 'Users',
        path: '/settings/users',
      },
      {
        id: 'groups',
        label: 'Groups',
        path: '/settings/groups',
      },
      {
        id: 'schedules',
        label: 'Schedules',
        path: '/settings/schedules',
      },
      {
        id: 'updates',
        label: 'Updates & releases',
        path: '/settings/updates',
      },
    ],
  },
]
