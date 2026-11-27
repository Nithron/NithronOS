import { useState, useEffect } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { AlertsIcon, ChevronDownIcon, ChevronRightIcon, CloseIcon, MenuIcon, SignOutIcon, UserIcon } from '@/components/icons'
import { navItems } from '@/config/nav'
import { Toasts } from '../ui/toast'
import { useAuth } from '@/lib/auth'
import { useQuery } from '@tanstack/react-query'
import { monitorApi } from '@/api/monitor'
import type { AlertEvent } from '@/api/monitor.types'
import { cn } from '@/lib/utils'
import { updatesApi } from '@/api/updates'
import { StatusMark } from '@/components/ui/pixel'

interface SidebarItemProps {
  item: typeof navItems[0]
  isActive?: boolean
  level?: number
}

function SidebarItem({ item, level = 0 }: SidebarItemProps) {
  const location = useLocation()
  const [expanded, setExpanded] = useState(false)
  const hasChildren = item.children && item.children.length > 0
  const isActive = location.pathname === item.path ||
    (hasChildren && item.children?.some(child => location.pathname === child.path))

  useEffect(() => {
    if (isActive && hasChildren) {
      setExpanded(true)
    }
  }, [isActive, hasChildren])

  const Icon = item.icon

  return (
    <div>
      <NavLink
        to={hasChildren ? '#' : item.path}
        onClick={(e) => {
          if (hasChildren) {
            e.preventDefault()
            setExpanded(!expanded)
          }
        }}
        // The current item: ink, weight 560 and a 2 px ink mark at its left;
        // the others in --ink-2. No fill.
        className={cn(
          'relative flex min-h-10 items-center gap-3 px-3 py-2 text-sm transition-colors',
          isActive
            ? 'font-[560] text-ink before:absolute before:inset-y-1 before:left-0 before:w-[2px] before:bg-ink'
            : 'text-ink-2 hover:bg-ink/[.09] hover:text-ink',
          level > 0 && 'ml-6'
        )}
      >
        {Icon && <Icon className="h-4 w-4" />}
        <span className="flex-1">{item.label}</span>
        {hasChildren && (
          <ChevronRightIcon
            className={cn(
              'h-4 w-4 transition-transform',
              expanded && 'rotate-90'
            )}
          />
        )}
      </NavLink>
      {hasChildren && expanded && (
        <div className="mt-1 space-y-1">
          {item.children!.map((child) => (
            <SidebarItem key={child.id} item={child} level={level + 1} />
          ))}
        </div>
      )}
    </div>
  )
}

export function AppShell({ children }: { children?: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [alertsOpen, setAlertsOpen] = useState(false)
  const { logout, session } = useAuth()
  // Firing alerts (admins only: the alert API is theirs). This polled
  // nothing every 5 s before.
  const isAdmin = Boolean(session?.user.isAdmin || session?.user.roles?.includes('admin'))
  const { data: alertEvents } = useQuery({
    queryKey: ['monitor', 'alerts', 'events'],
    queryFn: () => monitorApi.alerts.events.list(50),
    enabled: isAdmin,
    refetchInterval: 30_000,
  })
  const alerts: AlertEvent[] = (alertEvents?.events ?? []).filter((e) => e.state === 'firing')
  const { data: version } = useQuery({
    queryKey: ['updates', 'version'],
    queryFn: updatesApi.getVersion,
    staleTime: 10 * 60_000,
  })

  // Close user menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      if (!target.closest('[data-user-menu]')) {
        setUserMenuOpen(false)
      }
    }
    document.addEventListener('click', handleClickOutside)
    return () => {
      document.removeEventListener('click', handleClickOutside)
    }
  }, [])


  const handleLogout = async () => {
    try {
      await logout()
    } catch (error) {
      console.error('Logout failed:', error)
    }
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {/* Sidebar */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 w-64 transform border-r border-hairline bg-page transition-transform duration-300 ease-in-out lg:relative lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-full flex-col">
          {/* Sidebar Header */}
          <div className="flex h-14 items-center border-b border-hairline px-4">
            {/* The product's name in type (the pixel display face once it is
                issued); the drawn mark waits for the identity step. */}
            <Link to="/" className="pixel-display text-lg text-ink">
              nOS
            </Link>
            <button
              onClick={() => setSidebarOpen(false)}
              className="ml-auto lg:hidden"
              aria-label="Close sidebar"
            >
              <CloseIcon className="h-5 w-5" />
            </button>
          </div>

          {/* Sidebar Navigation */}
          <nav className="flex-1 space-y-1 overflow-y-auto p-4" aria-label="Main">
            {navItems.map((item) => (
              <SidebarItem key={item.id} item={item} />
            ))}
          </nav>

          {/* The version this system really runs (the Updates page's source). */}
          {version?.nosd_version && (
            <div className="border-t border-hairline p-4 font-mono text-xs text-ink-2">
              nOS {version.nosd_version}
            </div>
          )}
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <header className="flex h-14 items-center justify-between border-b border-hairline bg-page px-4">
          <button
            onClick={() => setSidebarOpen(true)}
            className="lg:hidden"
            aria-label="Open sidebar"
          >
            <MenuIcon className="h-5 w-5" />
          </button>

          <div className="flex-1" />

          <div className="flex items-center gap-4">
            {/* Alerts */}
            <button
              onClick={() => setAlertsOpen(true)}
              className="relative p-2 text-ink-2 hover:bg-ink/[.09] hover:text-ink"
              aria-label={alerts.length > 0 ? `Alerts, ${alerts.length} firing` : 'Alerts'}
            >
              <AlertsIcon className="h-4 w-4" />
              {alerts.length > 0 && (
                <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center bg-status-failed px-1 font-mono text-[10px] leading-none text-page">
                  {alerts.length}
                </span>
              )}
            </button>

            {/* User Menu */}
            <div className="relative" data-user-menu>
              <button
                onClick={() => setUserMenuOpen(!userMenuOpen)}
                className="flex items-center gap-2 p-2 text-sm text-ink-2 hover:bg-ink/[.09] hover:text-ink"
                aria-label="User menu"
              >
                <UserIcon className="h-4 w-4" />
                {session?.user && <span>{session.user.username}</span>}
                <ChevronDownIcon className="h-4 w-4" />
              </button>

              {userMenuOpen && (
                <div className="absolute right-0 top-full z-50 mt-2 w-48 border border-edge bg-page">
                  {session?.user && (
                    <div className="border-b border-hairline px-3 py-2">
                      <div className="text-sm font-medium">{session.user.username}</div>
                      <div className="text-xs text-ink-2">
                        {session.user.roles?.includes('admin') ? 'Administrator' : 'User'}
                      </div>
                    </div>
                  )}
                  <div className="p-1">
                    <button
                      onClick={handleLogout}
                      className="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-ink/[.09]"
                    >
                      <SignOutIcon className="h-4 w-4" />
                      Sign out
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 overflow-y-auto p-6">
          <Toasts />
          {children}
        </main>
      </div>

      {/* Alerts Panel */}
      {alertsOpen && (
        <div className="fixed inset-0 z-50 flex items-start justify-end bg-page/70">
          <div role="dialog" aria-label="Alerts" className="h-full w-full max-w-md overflow-auto border-l border-edge bg-page p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-[560]">Alerts</h2>
              <button
                onClick={() => setAlertsOpen(false)}
                aria-label="Close alerts"
              >
                <CloseIcon className="h-5 w-5" />
              </button>
            </div>
            {alerts.length === 0 ? (
              <p className="text-sm text-ink-2">No alert is firing.</p>
            ) : (
              <ul className="divide-y divide-hairline border-y border-hairline">
                {alerts.map((alert) => (
                  <li key={alert.id} className="py-3">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-[560]">{alert.rule_name}</span>
                      <StatusMark state={alert.severity === 'critical' ? 'failed' : 'warning'} className="text-xs">
                        {alert.severity}
                      </StatusMark>
                    </div>
                    <p className="mt-2 text-sm">{alert.message}</p>
                    <p className="mt-2 font-mono text-xs text-ink-2">
                      {new Date(alert.fired_at).toLocaleString()}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}

      {/* Mobile Sidebar Overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/30 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
    </div>
  )
}
