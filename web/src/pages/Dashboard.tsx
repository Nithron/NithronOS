import { motion, AnimatePresence } from 'framer-motion'
import { useState } from 'react'
import { ActivityIcon, AppIcon, CpuIcon, FailedIcon, FolderIcon, MaintenanceIcon, MemoryIcon, NoticeIcon, OkIcon, PoolIcon, QuickIcon, RefreshIcon, ShareIcon, StorageIcon, TimeIcon, WarningIcon } from '@/components/icons'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { ScrollArea } from '@/components/ui/scroll-area'
import { 
  useDashboard,
  useRefreshDashboard,
  formatTimestamp,
  getHealthBadgeVariant
} from '@/hooks/use-dashboard'
import { useSystemVitals, formatUptime } from '@/hooks/use-system-vitals'
import { bytesSafe, toFixedSafe } from '@/lib/format'
import { cn } from '@/lib/utils'
import { 
  PieChart, Pie, Cell, ResponsiveContainer, Tooltip
} from 'recharts'

// Animation variants
const containerVariants: any = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05
    }
  }
}

const itemVariants: any = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: {
      type: "spring",
      stiffness: 100
    }
  }
}

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
            System Health
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
            System Health
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
            System Health
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
  const StatusIcon = system.status === 'critical' ? FailedIcon :
                     system.status === 'degraded' ? WarningIcon : OkIcon

  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between items-start">
          <CardTitle className="flex items-center gap-2">
            <ActivityIcon className="h-5 w-5" />
            System Health
            {isSSE && (
              <Badge variant="outline" className="ml-auto text-xs">
                <QuickIcon className="h-3 w-3 mr-1" />
                Live
              </Badge>
            )}
          </CardTitle>
          <Badge variant={getHealthBadgeVariant(system.status as any)}>
            <StatusIcon className="h-3 w-3 mr-1" />
            {system.status || 'OK'}
          </Badge>
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
              initial={{ opacity: 0 }}
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
              initial={{ opacity: 0 }}
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
  
  const chartData = [
    { name: 'Used', value: storage.usedBytes || 0, fill: 'var(--primary)' },
    { name: 'Free', value: (storage.totalBytes || 0) - (storage.usedBytes || 0), fill: 'var(--muted)' }
  ]

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
        <div className="flex items-center gap-4">
          <div className="flex-1">
            <ResponsiveContainer width="100%" height={120}>
              <PieChart>
                <Pie
                  data={chartData}
                  cx="50%"
                  cy="50%"
                  innerRadius={35}
                  outerRadius={50}
                  paddingAngle={2}
                  dataKey="value"
                >
                  {chartData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.fill} />
                  ))}
                </Pie>
                <Tooltip formatter={(value: any) => bytesSafe(value)} />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="space-y-2">
            <div>
              <p className="text-sm text-muted-foreground">Used</p>
              <motion.p
                key={storage.usedBytes}
                initial={{ opacity: 0 }}
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
              <p className={cn(
                "text-lg font-semibold",
                usagePercent > 90 ? 'text-red-500' :
                usagePercent > 80 ? 'text-yellow-500' : ''
              )}>
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
            Disk Health
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
          Disk Health
        </CardTitle>
        <CardDescription>
          {disks.lastScanISO ? `SMART checked ${formatTimestamp(disks.lastScanISO)}` : 'Checking SMART…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Total Disks</span>
            <span className="font-medium">{disks.total || 0}</span>
          </div>
          
          <div className="flex gap-2">
            {disks.healthy > 0 && (
              <Badge variant="default" className="gap-1">
                <OkIcon className="h-3 w-3" />
                {disks.healthy} Healthy
              </Badge>
            )}
            {disks.warning > 0 && (
              <Badge variant="secondary" className="gap-1 bg-yellow-100">
                <WarningIcon className="h-3 w-3" />
                {disks.warning} Warning
              </Badge>
            )}
            {disks.critical > 0 && (
              <Badge variant="destructive" className="gap-1">
                <WarningIcon className="h-3 w-3" />
                {disks.critical} Critical
              </Badge>
            )}
            {disks.unknown > 0 && (
              <Badge variant="outline" className="gap-1">
                {disks.unknown} No SMART data
              </Badge>
            )}
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
            Recent Activity
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
          Recent Activity
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
                    initial={{ opacity: 0, x: -20 }}
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
            <div className="text-center py-8 text-muted-foreground">
              <TimeIcon className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No recent activity</p>
            </div>
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
            Network Shares
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
          Network Shares
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
          <div className="text-center py-4 text-muted-foreground">
            <FolderIcon className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">No shares configured</p>
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
            Installed Apps
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
          Installed Apps
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
          <div className="text-center py-4 text-muted-foreground">
            <AppIcon className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">No apps installed</p>
            <Button variant="outline" size="sm" className="mt-2">
              Browse Catalog
            </Button>
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
            Maintenance Operations
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
          Maintenance Operations
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">Pool Scrub</span>
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
          <h1 className="text-3xl font-bold">Dashboard</h1>
          <p className="text-muted-foreground">System overview and status</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={isRefreshing || isFetching}
        >
          <RefreshIcon className={cn("h-4 w-4 mr-2", (isRefreshing || isFetching) && "animate-spin")} />
          {isRefreshing ? 'Refreshing...' : 'Refresh'}
        </Button>
      </div>

      {/* Main Grid */}
      <motion.div
        variants={containerVariants}
        initial="hidden"
        animate="visible"
        className="grid gap-4 md:grid-cols-2 lg:grid-cols-4"
      >
        {/* System Health - spans 2 columns on large screens */}
        <motion.div variants={itemVariants} className="lg:col-span-2">
          <SystemHealthWidget data={data} isLoading={isLoading} />
        </motion.div>

        {/* Storage */}
        <motion.div variants={itemVariants}>
          <StorageWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>

        {/* Disk Health */}
        <motion.div variants={itemVariants}>
          <DiskHealthWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>

        {/* Recent Activity - spans 2 columns */}
        <motion.div variants={itemVariants} className="md:col-span-2">
          <RecentActivityWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>

        {/* Network Shares */}
        <motion.div variants={itemVariants}>
          <NetworkSharesWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>

        {/* Installed Apps */}
        <motion.div variants={itemVariants}>
          <InstalledAppsWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>

        {/* Maintenance Operations - spans 2 columns */}
        <motion.div variants={itemVariants} className="md:col-span-2">
          <MaintenanceWidget data={data} isLoading={isLoading} error={error} />
        </motion.div>
      </motion.div>
    </div>
  )
}