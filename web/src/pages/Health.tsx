import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { PageHeader } from '@/components/ui/page-header'
import { cn } from '@/lib/utils'
import { DiskIcon, HealthIcon, ServerIcon } from '@/components/icons'

export function Health() {
  const location = useLocation()
  const isBaseHealthPath = location.pathname === '/health'
  
  const tabs = [
    { 
      name: 'System', 
      href: '/health/system',
      icon: ServerIcon,
      description: 'System resource monitoring'
    },
    { 
      name: 'Disk', 
      href: '/health/disk',
      icon: DiskIcon,
      description: 'Disk health and SMART status'
    },
  ]
  
  return (
    <div className="space-y-6">
      <PageHeader
        title="Health monitoring"
        description="Monitor system and disk health status"
      />
      
      <div className="flex space-x-1 border-b">
        {tabs.map((tab) => {
          const Icon = tab.icon
          return (
            <NavLink
              key={tab.href}
              to={tab.href}
              className={({ isActive }) => cn(
                "flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors hover:text-primary",
                "border-b-2 -mb-[2px]",
                isActive
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground"
              )}
            >
              <Icon className="h-4 w-4" />
              {tab.name}
            </NavLink>
          )
        })}
      </div>
      
      {isBaseHealthPath ? (
        // Show system health by default when at /health
        <div className="text-center py-12">
          <HealthIcon className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium mb-2">Select a health category</h3>
          <p className="text-sm text-muted-foreground">
            Choose System or Disk health monitoring from the tabs above
          </p>
        </div>
      ) : (
        <Outlet />
      )}
    </div>
  )
}
