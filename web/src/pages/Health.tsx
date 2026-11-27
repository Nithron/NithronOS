import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { PageHeader } from '@/components/ui/page-header'
import { cn } from '@/lib/utils'

export function Health() {
  const location = useLocation()
  const isBaseHealthPath = location.pathname === '/health'
  
  const tabs = [
    { 
      name: 'System', 
      href: '/health/system',
      description: 'System resource monitoring'
    },
    { 
      name: 'Disk', 
      href: '/health/disk',
      description: 'Disk health and SMART status'
    },
  ]
  
  return (
    <div className="space-y-6">
      <PageHeader
        title="Health monitoring"
        description="Monitor system and disk health status"
      />
      
      {/* The same row of words as the Tabs component, as links. */}
      <nav aria-label="Health" className="flex items-end gap-6 border-b border-hairline">
        {tabs.map((tab) => (
          <NavLink
            key={tab.href}
            to={tab.href}
            className={({ isActive }) =>
              cn(
                'relative -mb-px inline-flex min-h-10 items-center py-2 text-sm transition-colors',
                isActive
                  ? 'font-[560] text-ink after:absolute after:inset-x-0 after:bottom-0 after:h-[2px] after:bg-ink'
                  : 'text-ink-2 hover:text-ink',
              )
            }
          >
            {tab.name}
          </NavLink>
        ))}
      </nav>
      
      {isBaseHealthPath ? (
        // Show system health by default when at /health
        <p className="py-6 text-sm text-ink-2">Choose system or disk health above.</p>
      ) : (
        <Outlet />
      )}
    </div>
  )
}
