import { createBrowserRouter, RouterProvider, Navigate, Outlet } from 'react-router-dom'
import { AppShell } from './components/layout/AppShell'
import { Dashboard } from './pages/Dashboard'
import { Storage } from './pages/Storage'
import { SharesList } from './pages/SharesList'
import { ShareDetails } from './pages/ShareDetails'
import { AppCatalog } from './pages/AppCatalog'
import { AppInstallWizard } from './pages/AppInstallWizard'
import { AppDetails } from './pages/AppDetails'
import { Settings } from './pages/Settings'
import { Users } from './pages/Users'
import SettingsSchedules from './routes/settings/schedules'
import { Remote } from './pages/Remote'
import { Login } from './pages/Login'
import { NetworkSettings } from './pages/NetworkSettings'
import { RemoteAccessWizard } from './pages/RemoteAccessWizard'
import { TwoFactorSettings } from './pages/TwoFactorSettings'
import { PoolsCreate } from './pages/PoolsCreate'
import { PoolDetails } from './pages/PoolDetails'
import Updates from './pages/Updates'
import SettingsSync from './pages/SettingsSync'
import SyncDevices from './pages/SyncDevices'
import SyncActivity from './pages/SyncActivity'
import SyncConflicts from './pages/SyncConflicts'
import Setup from './pages/Setup'
import Snapshots from './pages/Snapshots'
import BackupSchedules from './pages/BackupSchedules'
import MonitoringDashboard from './pages/MonitoringDashboard'
import { Health } from './pages/Health'
import SystemHealth from './pages/health/SystemHealth'
import DiskHealth from './pages/health/DiskHealth'
import { GlobalNoticeProvider, useGlobalNotice } from './lib/globalNotice'
import Banner from './components/Banner'
import HelpProxy from './pages/HelpProxy'
import { NotFound } from './pages/NotFound'
import { Toasts } from '@/components/ui/toast'
import { AuthProvider, AuthGuard } from './lib/auth'
import { useEffect, useState } from 'react'
import http, { ApiError } from './lib/nos-client'

// ============================================================================
// Root Layout with Auth Provider
// ============================================================================

function RootLayout() {
  const { notice } = useGlobalNotice()
  
  return (
    <AuthProvider>
      <div className="min-h-screen">
        {notice && (
          <Banner
            variant={notice.kind}
            title={notice.title}
            message={notice.message}
            action={notice.action}
          />
        )}
        <Outlet />
      </div>
    </AuthProvider>
  )
}

// ============================================================================
// Protected Layout Component
// ============================================================================

function ProtectedLayout() {
  return (
    <AuthGuard requireAuth={true}>
      <AppShell>
        <Outlet />
      </AppShell>
    </AuthGuard>
  )
}

// ============================================================================
// Setup Guard Component
// ============================================================================

function SetupGuard({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = useState(true)
  const [needsSetup, setNeedsSetup] = useState(false)
  
  useEffect(() => {
    checkSetupState()
  }, [])
  
  const checkSetupState = async () => {
    try {
      const state = await http.get<{firstBoot: boolean}>('/v1/setup/state')
      setNeedsSetup(state.firstBoot)
    } catch (err) {
      const error = err as ApiError
      if (error.status === 410) {
        // Setup complete
        setNeedsSetup(false)
      } else if (error.status === 502 || error.status === 503) {
        // Proxy misconfigured or backend down
        setNeedsSetup(false)
      } else {
        console.error('Setup state check failed:', err)
        setNeedsSetup(false)
      }
    } finally {
      setLoading(false)
    }
  }
  
  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }
  
  if (needsSetup) {
    return <Navigate to="/setup" replace />
  }
  
  return <>{children}</>
}

// ============================================================================
// Router Configuration
// ============================================================================

const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [
      {
        path: '/',
        element: <ProtectedLayout />,
        children: [
          { index: true, element: <Dashboard /> },
          { path: 'storage', element: <Storage /> },
          { path: 'shares', element: <SharesList /> },
          { path: 'shares/new', element: <ShareDetails /> },
          { path: 'shares/:name', element: <ShareDetails /> },
          { path: 'apps', element: <AppCatalog /> },
          { path: 'apps/install/:id', element: <AppInstallWizard /> },
          { path: 'apps/:id', element: <AppDetails /> },
          { 
            path: 'health', 
            element: <Health />,
            children: [
              { path: 'system', element: <SystemHealth /> },
              { path: 'disk', element: <DiskHealth /> },
            ]
          },
          { path: 'settings', element: <Settings /> },
          { path: 'settings/users', element: <Users /> },
          { path: 'settings/schedules', element: <SettingsSchedules /> },
          { path: 'settings/updates', element: <Updates /> },
          { path: 'settings/network', element: <NetworkSettings /> },
          { path: 'backup/snapshots', element: <Snapshots /> },
          { path: 'backup/schedules', element: <BackupSchedules /> },
          { path: 'monitor', element: <MonitoringDashboard /> },
          { path: 'settings/network/wizard', element: <RemoteAccessWizard /> },
          { path: 'settings/2fa', element: <TwoFactorSettings /> },
          { path: 'settings/sync', element: <SettingsSync /> },
          { path: 'settings/sync/devices', element: <SyncDevices /> },
          { path: 'settings/sync/activity', element: <SyncActivity /> },
          { path: 'settings/sync/conflicts', element: <SyncConflicts /> },
          { path: 'remote', element: <Remote /> },
          { path: 'storage/create', element: <PoolsCreate /> },
          { path: 'storage/:id', element: <PoolDetails /> },
          // Redirect old schedules route to new location
          { path: 'schedules', element: <Navigate to="/settings/schedules" replace /> },
        ],
      },
      {
        path: '/login',
        element: (
          <SetupGuard>
            <Login />
          </SetupGuard>
        ),
      },
      {
        path: '/setup',
        element: <Setup />,
      },
      {
        path: '/help/proxy',
        element: <HelpProxy />,
      },
      {
        path: '*',
        element: <NotFound />,
      },
    ],
  },
])

// ============================================================================
// Main App Component
// ============================================================================

export default function App() {
  return (
    <GlobalNoticeProvider>
      <Toasts />
      <RouterProvider router={router} />
    </GlobalNoticeProvider>
  )
}