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
        className={cn(
          'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-all hover:bg-accent/10',
          isActive && 'bg-primary/10 text-primary',
          !isActive && 'text-muted-foreground hover:text-foreground',
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
          'fixed inset-y-0 left-0 z-50 w-64 transform bg-card transition-transform duration-300 ease-in-out lg:relative lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-full flex-col">
          {/* Sidebar Header */}
          <div className="flex h-14 items-center border-b border-border px-4">
            <Link to="/" className="flex items-center gap-2">
              <img 
                src="/brand/nos-logo-mark.svg" 
                alt="nOS Logo" 
                className="h-8 w-8"
              />
              <span className="text-lg font-semibold">nOS</span>
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
          <nav className="flex-1 space-y-1 overflow-y-auto p-4">
            {navItems.map((item) => (
              <SidebarItem key={item.id} item={item} />
            ))}
          </nav>

          {/* Sidebar Footer */}
          <div className="border-t border-border p-4">
            <div className="text-xs text-muted-foreground">
              v1.0.0 • © 2024 nOS
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <header className="flex h-14 items-center justify-between border-b border-border bg-card px-4">
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
              className="relative rounded-lg p-2 hover:bg-accent/10"
              aria-label="View alerts"
            >
              <AlertsIcon className="h-5 w-5 text-muted-foreground" />
              {alerts.length > 0 && (
                <span className="absolute -right-1 -top-1 flex h-5 min-w-[20px] items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-medium text-destructive-foreground">
                  {alerts.length}
                </span>
              )}
            </button>

            {/* User Menu */}
            <div className="relative" data-user-menu>
              <button
                onClick={() => setUserMenuOpen(!userMenuOpen)}
                className="flex items-center gap-2 rounded-lg p-2 hover:bg-accent/10"
                aria-label="User menu"
              >
                <UserIcon className="h-5 w-5 text-muted-foreground" />
                <ChevronDownIcon className="h-4 w-4 text-muted-foreground" />
              </button>

              {userMenuOpen && (
                <div className="absolute right-0 top-full mt-2 w-48 rounded-lg border border-border bg-card shadow-lg">
                  {session?.user && (
                    <div className="border-b border-border px-3 py-2">
                      <div className="text-sm font-medium">{session.user.username}</div>
                      <div className="text-xs text-muted-foreground">
                        {session.user.roles?.includes('admin') ? 'Administrator' : 'User'}
                      </div>
                    </div>
                  )}
                  <div className="p-1">
                    <button
                      onClick={handleLogout}
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent/10"
                    >
                      <SignOutIcon className="h-4 w-4" />
                      Sign Out
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
        <div className="fixed inset-0 z-50 flex items-start justify-end bg-black/30">
          <div className="h-full w-full max-w-md overflow-auto bg-card p-6 shadow-lg">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">System Alerts</h2>
              <button
                onClick={() => setAlertsOpen(false)}
                aria-label="Close alerts"
              >
                <CloseIcon className="h-5 w-5" />
              </button>
            </div>
            {alerts.length === 0 ? (
              <div className="text-center text-sm text-muted-foreground">
                No active alerts
              </div>
            ) : (
              <div className="space-y-3">
                {alerts.map((alert) => (
                  <div
                    key={alert.id}
                    className={cn(
                      'rounded-lg border p-3',
                      alert.severity === 'critical'
                        ? 'border-destructive bg-destructive/10'
                        : 'border-yellow-600 bg-yellow-600/10'
                    )}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">{alert.rule_name}</span>
                      <span
                        className={cn(
                          'rounded px-2 py-0.5 text-xs font-medium',
                          alert.severity === 'critical'
                            ? 'bg-destructive text-destructive-foreground'
                            : 'bg-yellow-600 text-yellow-50'
                        )}
                      >
                        {alert.severity}
                      </span>
                    </div>
                    <div className="mt-2 text-sm">
                      {alert.message}
                    </div>
                    <div className="mt-2 text-xs text-muted-foreground">
                      {new Date(alert.fired_at).toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
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
