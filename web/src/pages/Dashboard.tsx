import { motion, AnimatePresence } from 'framer-motion'
import { useState } from 'react'
import { WorkingIcon, ActivityIcon, AppIcon, CpuIcon, FailedIcon, FolderIcon, MaintenanceIcon, MemoryIcon, NoticeIcon, PoolIcon, QuickIcon, RefreshIcon, ShareIcon, StorageIcon, WarningIcon } from '@/components/icons'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button, buttonVariants } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { ScrollArea } from '@/components/ui/scroll-area'
import { 
  useDashboard,
  useRefreshDashboard,
  formatTimestamp,
} from '@/hooks/use-dashboard'
import { useSystemVitals, formatUptime } from '@/hooks/use-system-vitals'
import { bytesSafe, toFixedSafe } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Link } from 'react-router-dom'
import { Meter, StatusMark, type MarkState } from '@/components/ui/pixel'

// The overall health as a mark: critical is failed, degraded a warning.
const healthMark = (status?: string): MarkState =>
  status === 'critical' ? 'failed' : status === 'degraded' ? 'warning' : 'ok'

// Widget components
function SystemHealthWidget({ data: dashboardData, isLoading: dashboardLoading }: any) {
  // Use system vitals for real-time updates, fall back to dashboard data
  const { vitals, isLoading: vitalsLoading, error, isSSE } = useSystemVitals()
  
  // Prefer vitals if available, otherwise use dashboard data
  const systemData = vitals ? {
    status: vitals.cpuPct > 80 ? 'critical' : vitals.cpuPct > 60 ? 'degraded' : 'ok',
    cpuPct: vitals.cpuPct || 0,
    mem: { 
      used: vitals.memUsed || 0, 
      total: vitals.memTotal || 1 
    },
    swap: { 
      used: vitals.swapUsed || 0, 
      total: vitals.swapTotal || 0 
    },
    load1: vitals.load1 || 0,
    uptimeSec: vitals.uptime || 0,
  } : dashboardData?.system ? {
    status: dashboardData.system.status || 'ok',
    cpuPct: dashboardData.system.cpuPct || 0,
    mem: dashboardData.system.mem || { used: 0, total: 1 },
    swap: dashboardData.system.swap || { used: 0, total: 0 },
    load1: dashboardData.system.load1 || 0,
    uptimeSec: dashboardData.system.uptimeSec || 0,
  } : null
  
  const isLoading = vitalsLoading && dashboardLoading
  const data = systemData ? { system: systemData } : null
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            System health
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-2 w-full" />
            <Skeleton className="h-2 w-full" />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (error && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            System health
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive">
            <FailedIcon className="h-4 w-4" />
            <AlertDescription>Data unavailable</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  if (!data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            System health
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Alert>
            <NoticeIcon className="h-4 w-4" />
            <AlertDescription>Waiting for data...</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  const system = data.system

  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between items-start">
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            System health
            {isSSE && (
              <StatusMark state="running" className="ml-auto text-xs font-normal">
                Live
              </StatusMark>
            )}
          </CardTitle>
          <StatusMark state={healthMark(system.status)} className="text-sm">
            {system.status || 'ok'}
          </StatusMark>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* CPU Usage */}
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className="flex items-center gap-1">
              <CpuIcon className="h-3 w-3" />
              CPU
            </span>
            <motion.span
              key={system.cpuPct}
              initial={false}
              animate={{ opacity: 1 }}
              className={cn(
                "font-medium",
                system.cpuPct > 80 ? 'text-red-500' : 
                system.cpuPct > 60 ? 'text-yellow-500' : ''
              )}
            >
              {Math.round(system.cpuPct || 0)}%
            </motion.span>
          </div>
          <Progress 
            value={system.cpuPct || 0} 
            className="h-2"
            indicatorClassName={cn(
              "transition-all duration-300",
              system.cpuPct > 80 ? 'bg-red-500' : 
              system.cpuPct > 60 ? 'bg-yellow-500' : ''
            )}
          />
        </div>

        {/* Memory Usage */}
        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className="flex items-center gap-1">
              <MemoryIcon className="h-3 w-3" />
              Memory
            </span>
            <motion.span
              key={system.mem?.used}
              initial={false}
              animate={{ opacity: 1 }}
              className="font-medium"
            >
              {bytesSafe(system.mem?.used)} / {bytesSafe(system.mem?.total)}
            </motion.span>
          </div>
          <Progress 
            value={system.mem?.total ? (system.mem.used / system.mem.total) * 100 : 0} 
            className="h-2"
            indicatorClassName={cn(
              "transition-all duration-300",
              system.mem?.total && (system.mem.used / system.mem.total) > 0.8 ? 'bg-red-500' : 
              system.mem?.total && (system.mem.used / system.mem.total) > 0.6 ? 'bg-yellow-500' : ''
            )}
          />
        </div>

        {/* Load Average */}
        <div className="flex justify-between items-center text-sm">
          <span className="text-muted-foreground">Load</span>
          <span className="font-medium">{toFixedSafe(system.load1, 2, '0.00')}</span>
        </div>

        {/* Uptime */}
        <div className="flex justify-between items-center text-sm">
          <span className="text-muted-foreground">Uptime</span>
          <span className="font-medium">{formatUptime(system.uptimeSec || 0)}</span>
        </div>
      </CardContent>
    </Card>
  )
}

function StorageWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <PoolIcon className="h-5 w-5" />
            Storage
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-32 w-full" />
        </CardContent>
      </Card>
    )
  }

  const storage = data?.storage || {}
  const usagePercent = storage.totalBytes ? (storage.usedBytes / storage.totalBytes) * 100 : 0
  

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <PoolIcon className="h-5 w-5" />
          Storage
        </CardTitle>
        <CardDescription>
          {storage.poolsOnline || 0} of {storage.poolsTotal || 0} pools online
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          <div>
            <Meter
              value={storage.usedBytes || 0}
              max={storage.totalBytes || 0}
              blocks={12}
              label="Storage used"
              valueText={`${toFixedSafe(usagePercent, 1, '0.0')}%`}
              tone={usagePercent > 90 ? 'failed' : usagePercent > 80 ? 'warning' : 'ink'}
            />
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <p className="text-sm text-muted-foreground">Used</p>
              <motion.p
                key={storage.usedBytes}
                initial={false}
                animate={{ opacity: 1 }}
                className="text-lg font-semibold"
              >
                {bytesSafe(storage.usedBytes)}
              </motion.p>
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Total</p>
              <p className="text-lg font-semibold">
                {bytesSafe(storage.totalBytes)}
              </p>
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Usage</p>
              <p className="text-lg font-semibold">
                {toFixedSafe(usagePercent, 1, '0.0')}%
              </p>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function DiskHealthWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <StorageIcon className="h-5 w-5" />
            Disk health
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-32" />
          </div>
        </CardContent>
      </Card>
    )
  }

  // Guard against missing data or non-array
  const disks = data?.disks || {}

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <StorageIcon className="h-5 w-5" />
          Disk health
        </CardTitle>
        <CardDescription>
          {disks.lastScanISO ? `SMART checked ${formatTimestamp(disks.lastScanISO)}` : 'Checking SMART…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Total disks</span>
            <span className="font-medium">{disks.total || 0}</span>
          </div>
          
          <div className="flex gap-2">
            {disks.healthy > 0 && <StatusMark state="ok" className="text-sm">{disks.healthy} healthy</StatusMark>}
            {disks.warning > 0 && <StatusMark state="warning" className="text-sm">{disks.warning} warning</StatusMark>}
            {disks.critical > 0 && <StatusMark state="failed" className="text-sm">{disks.critical} failing</StatusMark>}
            {disks.unknown > 0 && <StatusMark state="off" className="text-sm">{disks.unknown} without SMART data</StatusMark>}
            {disks.lastScanISO && disks.total === 0 && (
              <span className="text-sm text-muted-foreground">No disks detected</span>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function RecentActivityWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            Recent activity
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        </CardContent>
      </Card>
    )
  }

  // Guard against non-array
  const events = Array.isArray(data?.events) ? data.events : []

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'error':
        return <WarningIcon className="h-4 w-4 text-red-500" />
      case 'warning':
        return <WarningIcon className="h-4 w-4 text-yellow-500" />
      default:
        return <NoticeIcon className="h-4 w-4 text-blue-500" />
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ActivityIcon className="h-5 w-5" />
          Recent activity
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-[180px]">
          {events.length > 0 ? (
            <div className="space-y-2">
              <AnimatePresence mode="popLayout">
                {events.slice(0, 10).map((event: any) => (
                  <motion.div
                    key={event.id}
                    layout
                    initial={false}
                    animate={{ opacity: 1, x: 0 }}
                    exit={{ opacity: 0, x: 20 }}
                    className="flex items-start gap-2 p-2 rounded-lg hover:bg-muted/50 transition-colors"
                  >
                    {getSeverityIcon(event.severity)}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm truncate">{event.message}</p>
                      <p className="text-xs text-muted-foreground">
                        {formatTimestamp(event.timestamp)}
                      </p>
                    </div>
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          ) : (
            <p className="py-4 text-sm text-ink-2">Nothing has happened yet.</p>
          )}
        </ScrollArea>
      </CardContent>
    </Card>
  )
}

function NetworkSharesWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ShareIcon className="h-5 w-5" />
            Network shares
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-20 w-full" />
        </CardContent>
      </Card>
    )
  }

  // Guard against non-array
  const shares = Array.isArray(data?.shares) ? data.shares : []
  const activeShares = shares.filter((s: any) => s.state === 'active')

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ShareIcon className="h-5 w-5" />
          Network shares
        </CardTitle>
        <CardDescription>
          {activeShares.length} of {shares.length} active
        </CardDescription>
      </CardHeader>
      <CardContent>
        {shares.length > 0 ? (
          <div className="space-y-2">
            {shares.slice(0, 3).map((share: any) => (
              <div key={share.name} className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <FolderIcon className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">{share.name}</span>
                  <Badge variant="outline" className="text-xs">
                    {share.proto}
                  </Badge>
                </div>
                <Badge variant={share.state === 'active' ? 'default' : 'secondary'}>
                  {share.state}
                </Badge>
              </div>
            ))}
            {shares.length > 3 && (
              <p className="text-xs text-muted-foreground text-center">
                +{shares.length - 3} more
              </p>
            )}
          </div>
        ) : (
          <div className="space-y-3 py-4">
            <p className="text-sm text-ink-2">No share yet.</p>
            <Link to="/shares/new" className={buttonVariants({ variant: 'outline', size: 'sm' })}>
              Create a share
            </Link>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function InstalledAppsWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <AppIcon className="h-5 w-5" />
            Installed apps
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-20 w-full" />
        </CardContent>
      </Card>
    )
  }

  // Guard against non-array
  const apps = Array.isArray(data?.apps) ? data.apps : []
  const runningApps = apps.filter((a: any) => a.state === 'running')

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <AppIcon className="h-5 w-5" />
          Installed apps
        </CardTitle>
        <CardDescription>
          {runningApps.length} of {apps.length} running
        </CardDescription>
      </CardHeader>
      <CardContent>
        {apps.length > 0 ? (
          <div className="space-y-2">
            {apps.slice(0, 3).map((app: any) => (
              <div key={app.id} className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <AppIcon className="h-4 w-4 text-muted-foreground" />
                  <div>
                    <p className="text-sm font-medium">{app.name}</p>
                    <p className="text-xs text-muted-foreground">{app.version}</p>
                  </div>
                </div>
                <Badge variant={app.state === 'running' ? 'default' : 'secondary'}>
                  {app.state}
                </Badge>
              </div>
            ))}
            {apps.length > 3 && (
              <p className="text-xs text-muted-foreground text-center">
                +{apps.length - 3} more
              </p>
            )}
          </div>
        ) : (
          <div className="space-y-3 py-4">
            <p className="text-sm text-ink-2">No app installed.</p>
            <Link to="/apps" className={buttonVariants({ variant: 'outline', size: 'sm' })}>
              Browse the catalog
            </Link>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MaintenanceWidget({ data, isLoading }: any) {
  if (isLoading && !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <MaintenanceIcon className="h-5 w-5" />
            Maintenance operations
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    )
  }

  const maintenance = data?.maintenance || {}
  
  const getStatusBadge = (state: string) => {
    switch (state) {
      case 'running':
        return <Badge variant="default" className="gap-1">
          <QuickIcon className="h-3 w-3" />
          Running
        </Badge>
      case 'scheduled':
        return <Badge variant="secondary">Scheduled</Badge>
      default:
        return <Badge variant="outline">Idle</Badge>
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <MaintenanceIcon className="h-5 w-5" />
          Maintenance operations
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Pool scrub</span>
            {getStatusBadge(maintenance.scrub?.state || 'idle')}
          </div>
          {maintenance.scrub?.nextISO && maintenance.scrub.state !== 'running' && (
            <p className="text-xs text-muted-foreground">
              Next: {formatTimestamp(maintenance.scrub.nextISO)}
            </p>
          )}
          
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Balance</span>
            {getStatusBadge(maintenance.balance?.state || 'idle')}
          </div>
          {maintenance.balance?.nextISO && maintenance.balance.state !== 'running' && (
            <p className="text-xs text-muted-foreground">
              Next: {formatTimestamp(maintenance.balance.nextISO)}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export function Dashboard() {
  const { data, isLoading, error, isFetching, refetch } = useDashboard()
  const refreshDashboard = useRefreshDashboard()
  const [isRefreshing, setIsRefreshing] = useState(false)
  
  const handleRefresh = async () => {
    setIsRefreshing(true)
    await refetch()
    refreshDashboard()
    setTimeout(() => setIsRefreshing(false), 500) // Show refresh state briefly
  }

  return (
    <div className="space-y-6 p-6">
      {error && (
        <Alert variant="destructive">
          <FailedIcon className="h-4 w-4" />
          <AlertDescription>
            Backend unreachable or proxy misconfigured — metrics temporarily unavailable.{' '}
            <a href="/docs/dev/observability" className="underline">Troubleshooting</a>
          </AlertDescription>
        </Alert>
      )}
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="screen-title">Dashboard</h1>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={isRefreshing || isFetching}
        >
          {isRefreshing || isFetching ? <WorkingIcon className="h-4 w-4" /> : <RefreshIcon className="h-4 w-4" />}
          {isRefreshing || isFetching ? 'Refreshing' : 'Refresh'}
        </Button>
      </div>

      {/* Main Grid */}
      {/* One ruled grid of equal cells: thirds, then the full width. The
          gaps are the hairline behind the cells; the cards lose their own
          border inside it. */}
      <div className="grid gap-px border border-hairline bg-hairline md:grid-cols-2 lg:grid-cols-3 [&>*]:bg-page [&>*>*]:h-full [&>*>*]:border-0">
        <div>
          <SystemHealthWidget data={data} isLoading={isLoading} />
        </div>
        <div>
          <StorageWidget data={data} isLoading={isLoading} error={error} />
        </div>
        <div>
          <DiskHealthWidget data={data} isLoading={isLoading} error={error} />
        </div>
        <div>
          <RecentActivityWidget data={data} isLoading={isLoading} error={error} />
        </div>
        <div>
          <NetworkSharesWidget data={data} isLoading={isLoading} error={error} />
        </div>
        <div>
          <InstalledAppsWidget data={data} isLoading={isLoading} error={error} />
        </div>
        <div className="md:col-span-2 lg:col-span-3">
          <MaintenanceWidget data={data} isLoading={isLoading} error={error} />
        </div>
      </div>
    </div>
  )
}