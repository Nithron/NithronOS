import { useServices } from '@/api/system'
import { getErrorMessage } from '@/lib/nos-client'
import { motion } from 'framer-motion'
import { useState, useMemo, useEffect, useRef } from 'react'
import { CpuIcon, DiskIcon, FailedIcon, LiveIcon, MemoryIcon, NetworkIcon, NoticeIcon, OkIcon, RefreshIcon, RisingIcon } from '@/components/icons'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useMonitoringData, formatBytes, formatUptime } from '@/hooks/use-health'
import { useQueryClient } from '@tanstack/react-query'
import { toFixedSafe } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend
} from 'recharts'

// Animation variants
const containerVariants: any = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.1
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

// Chart data buffer (keep last 60 seconds)
const MAX_DATA_POINTS = 60

interface ChartData {
  time: string
  cpu: number
  memory: number
  network_rx: number
  network_tx: number
  disk_read: number
  disk_write: number
}

// Live metric component
function LiveMetric({ 
  label, 
  value, 
  unit = '', 
  icon: Icon,
  trend,
  color = 'text-primary'
}: {
  label: string
  value: string | number
  unit?: string
  icon: React.ElementType
  trend?: 'up' | 'down' | 'stable'
  color?: string
}) {
  return (
    <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
      <div className="flex items-center gap-2">
        <Icon className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm text-muted-foreground">{label}</span>
      </div>
      <motion.div
        key={value}
        initial={{ opacity: 0, y: -5 }}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center gap-1"
      >
        <span className={cn("font-semibold", color)}>{value}</span>
        {unit && <span className="text-sm text-muted-foreground">{unit}</span>}
        {trend === 'up' && <RisingIcon className="h-3 w-3 text-green-500" />}
      </motion.div>
    </div>
  )
}

// Service status card
function ServiceCard({ 
  name, 
  status, 
  description 
}: {
  name: string
  status: 'running' | 'stopped' | 'error' | 'unknown'
  description: string
}) {
  const statusConfig = {
    running: { color: 'text-green-500', icon: OkIcon, label: 'Running' },
    stopped: { color: 'text-gray-500', icon: FailedIcon, label: 'Stopped' },
    error: { color: 'text-red-500', icon: FailedIcon, label: 'Error' },
    unknown: { color: 'text-yellow-500', icon: NoticeIcon, label: 'Unknown' }
  }

  const config = statusConfig[status] || statusConfig.unknown
  const Icon = config.icon

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex justify-between items-start">
          <div>
            <CardTitle className="text-sm font-medium">{name}</CardTitle>
            <CardDescription className="text-xs mt-1">{description}</CardDescription>
          </div>
          <Badge variant="outline" className={cn("gap-1", config.color)}>
            <Icon className="h-3 w-3" />
            {config.label}
          </Badge>
        </div>
      </CardHeader>
    </Card>
  )
}


// What each systemd service is, in words.
const serviceLabels: Record<string, { name: string; description: string }> = {
  nosd: { name: 'nOS daemon', description: 'The web interface and API' },
  'nos-agent': { name: 'nOS agent', description: 'Does the privileged work for nosd' },
  caddy: { name: 'Caddy', description: 'HTTPS and the reverse proxy' },
  docker: { name: 'Docker', description: 'Runs the installed apps' },
  smbd: { name: 'Samba', description: 'Windows and macOS file sharing (SMB)' },
  'nfs-server': { name: 'NFS server', description: 'Linux and Unix file sharing' },
}

export default function MonitoringDashboard() {
  const queryClient = useQueryClient()
  const { data: metrics, isLoading, error, refetch } = useMonitoringData()
  const [chartData, setChartData] = useState<ChartData[]>([])
  const [isManualRefreshing, setIsManualRefreshing] = useState(false)
  const chartDataRef = useRef<ChartData[]>([])

  // Update chart data when metrics change
  useEffect(() => {
    if (!metrics) return

    const now = new Date()
    const timeStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
    
    const newPoint: ChartData = {
      time: timeStr,
      cpu: metrics.cpu,
      memory: metrics.memory.usagePct,
      network_rx: metrics.network.rxSpeed,
      network_tx: metrics.network.txSpeed,
      disk_read: metrics.diskIO.readSpeed,
      disk_write: metrics.diskIO.writeSpeed
    }

    chartDataRef.current = [...chartDataRef.current, newPoint].slice(-MAX_DATA_POINTS)
    setChartData(chartDataRef.current)
  }, [metrics])

  // Calculate statistics
  const stats = useMemo(() => {
    if (!metrics) return null

    return {
      avgCpu: metrics.cpu,
      peakCpu: Math.max(...chartData.map(d => d.cpu), metrics.cpu),
      avgMemory: metrics.memory.usagePct,
      totalNetworkRx: metrics.network.bytesRecv,
      totalNetworkTx: metrics.network.bytesSent,
      totalDiskRead: metrics.diskIO.readBytes,
      totalDiskWrite: metrics.diskIO.writeBytes,
      uptime: metrics.uptimeSec
    }
  }, [metrics, chartData])

  const handleManualRefresh = async () => {
    setIsManualRefreshing(true)
    await queryClient.invalidateQueries({ queryKey: ['monitor', 'system'] })
    await refetch()
    setTimeout(() => setIsManualRefreshing(false), 1000)
  }

  // The services' real state from systemd. (This was a fixed list that
  // always showed SMB "stopped" and everything else "running".)
  const { data: serviceStates, error: servicesError } = useServices()
  const services = (serviceStates ?? []).map((s) => ({
    name: serviceLabels[s.name]?.name ?? s.name,
    description: serviceLabels[s.name]?.description ?? s.name,
    status: s.status === 'failed' ? ('error' as const) : s.status,
  }))

  if (isLoading && !metrics) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h2 className="text-2xl font-bold">System Monitoring</h2>
        </div>
        <div className="grid gap-4">
          {[...Array(3)].map((_, i) => (
            <Card key={i} className="animate-pulse">
              <CardHeader>
                <div className="h-4 bg-muted rounded w-32"></div>
              </CardHeader>
              <CardContent>
                <div className="h-32 bg-muted rounded"></div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <Alert variant="destructive">
          <FailedIcon className="h-4 w-4" />
          <AlertDescription>
            Backend unreachable or proxy misconfigured — metrics temporarily unavailable. 
            <Button 
              variant="link" 
              className="px-2"
              onClick={() => refetch()}
            >
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    )
  }

  return (
    <motion.div
      variants={containerVariants}
      initial="hidden"
      animate="visible"
      className="space-y-6"
    >
      {/* Header */}
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <h2 className="text-2xl font-bold">System Monitoring</h2>
          <Badge variant="outline" className="gap-1">
            <LiveIcon className="h-3 w-3 text-green-500 animate-pulse" />
            Live
          </Badge>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">
            Uptime: {formatUptime(stats?.uptime || 0)}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={handleManualRefresh}
            disabled={isManualRefreshing}
          >
            <RefreshIcon className={cn("h-4 w-4 mr-2", isManualRefreshing && "animate-spin")} />
            Refresh
          </Button>
        </div>
      </div>

      {/* Live metrics grid */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Live Metrics</CardTitle>
            <CardDescription>Real-time system performance (1Hz refresh)</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <LiveMetric
                label="CPU"
                value={`${Math.round(metrics?.cpu || 0)}%`}
                icon={CpuIcon}
                color={metrics?.cpu && metrics.cpu > 80 ? 'text-red-500' : ''}
              />
              <LiveMetric
                label="Memory"
                value={`${Math.round(metrics?.memory?.usagePct || 0)}%`}
                icon={MemoryIcon}
                color={metrics?.memory?.usagePct && metrics.memory.usagePct > 80 ? 'text-red-500' : ''}
              />
              <LiveMetric
                label="Network"
                value={formatBytes((metrics?.network?.rxSpeed || 0) + (metrics?.network?.txSpeed || 0))}
                unit="/s"
                icon={NetworkIcon}
              />
              <LiveMetric
                label="Disk I/O"
                value={formatBytes((metrics?.diskIO?.readSpeed || 0) + (metrics?.diskIO?.writeSpeed || 0))}
                unit="/s"
                icon={DiskIcon}
              />
            </div>
          </CardContent>
        </Card>
      </motion.div>

      {/* Charts */}
      <motion.div variants={itemVariants}>
        <Tabs defaultValue="cpu" className="space-y-4">
          <TabsList className="grid w-full grid-cols-4">
            <TabsTrigger value="cpu">CPU & Memory</TabsTrigger>
            <TabsTrigger value="network">Network</TabsTrigger>
            <TabsTrigger value="disk">Disk I/O</TabsTrigger>
            <TabsTrigger value="services">Services</TabsTrigger>
          </TabsList>

          <TabsContent value="cpu" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">CPU & Memory Usage</CardTitle>
                <CardDescription>Last 60 seconds</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis 
                      dataKey="time" 
                      className="text-xs"
                      interval="preserveStartEnd"
                    />
                    <YAxis 
                      domain={[0, 100]}
                      className="text-xs"
                      tickFormatter={(value) => `${toFixedSafe(value, 0, '0')}%`}
                    />
                    <Tooltip 
                      contentStyle={{ 
                        backgroundColor: 'hsl(var(--background))',
                        border: '1px solid hsl(var(--border))'
                      }}
                      formatter={(value) => `${toFixedSafe(Number(value), 1, '0.0')}%`}
                    />
                    <Legend />
                    <Line 
                      type="monotone" 
                      dataKey="cpu" 
                      stroke="hsl(var(--primary))" 
                      strokeWidth={2}
                      dot={false}
                      name="CPU"
                    />
                    <Line 
                      type="monotone" 
                      dataKey="memory" 
                      stroke="hsl(var(--chart-2))" 
                      strokeWidth={2}
                      dot={false}
                      name="Memory"
                    />
                  </LineChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="network" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Network Traffic</CardTitle>
                <CardDescription>Upload and download speeds</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <AreaChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis 
                      dataKey="time" 
                      className="text-xs"
                      interval="preserveStartEnd"
                    />
                    <YAxis 
                      className="text-xs"
                      tickFormatter={(value) => formatBytes(value) + '/s'}
                    />
                    <Tooltip 
                      contentStyle={{ 
                        backgroundColor: 'hsl(var(--background))',
                        border: '1px solid hsl(var(--border))'
                      }}
                      formatter={(value) => formatBytes(Number(value)) + '/s'}
                    />
                    <Legend />
                    <Area 
                      type="monotone" 
                      dataKey="network_rx" 
                      stackId="1"
                      stroke="hsl(var(--chart-3))" 
                      fill="hsl(var(--chart-3))"
                      fillOpacity={0.6}
                      name="Download"
                    />
                    <Area 
                      type="monotone" 
                      dataKey="network_tx" 
                      stackId="1"
                      stroke="hsl(var(--chart-4))" 
                      fill="hsl(var(--chart-4))"
                      fillOpacity={0.6}
                      name="Upload"
                    />
                  </AreaChart>
                </ResponsiveContainer>
                
                <div className="grid grid-cols-2 gap-4 mt-4 pt-4 border-t">
                  <div>
                    <p className="text-sm text-muted-foreground">Total Downloaded</p>
                    <p className="text-lg font-semibold">{formatBytes(stats?.totalNetworkRx || 0)}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Total Uploaded</p>
                    <p className="text-lg font-semibold">{formatBytes(stats?.totalNetworkTx || 0)}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="disk" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Disk I/O</CardTitle>
                <CardDescription>Read and write operations</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <AreaChart data={chartData}>
                    <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
                    <XAxis 
                      dataKey="time" 
                      className="text-xs"
                      interval="preserveStartEnd"
                    />
                    <YAxis 
                      className="text-xs"
                      tickFormatter={(value) => formatBytes(value) + '/s'}
                    />
                    <Tooltip 
                      contentStyle={{ 
                        backgroundColor: 'hsl(var(--background))',
                        border: '1px solid hsl(var(--border))'
                      }}
                      formatter={(value) => formatBytes(Number(value)) + '/s'}
                    />
                    <Legend />
                    <Area 
                      type="monotone" 
                      dataKey="disk_read" 
                      stackId="1"
                      stroke="hsl(var(--chart-1))" 
                      fill="hsl(var(--chart-1))"
                      fillOpacity={0.6}
                      name="Read"
                    />
                    <Area 
                      type="monotone" 
                      dataKey="disk_write" 
                      stackId="1"
                      stroke="hsl(var(--chart-2))" 
                      fill="hsl(var(--chart-2))"
                      fillOpacity={0.6}
                      name="Write"
                    />
                  </AreaChart>
                </ResponsiveContainer>
                
                <div className="grid grid-cols-2 gap-4 mt-4 pt-4 border-t">
                  <div>
                    <p className="text-sm text-muted-foreground">Total Read</p>
                    <p className="text-lg font-semibold">{formatBytes(stats?.totalDiskRead || 0)}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Total Written</p>
                    <p className="text-lg font-semibold">{formatBytes(stats?.totalDiskWrite || 0)}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="services" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">System Services</CardTitle>
                <CardDescription>Status of core services</CardDescription>
              </CardHeader>
              <CardContent>
                {servicesError && (
                  <p className="text-sm text-destructive">The service states could not be read: {getErrorMessage(servicesError)}</p>
                )}
                <div className="grid gap-3 md:grid-cols-2">
                  {services.map((service) => (
                    <ServiceCard
                      key={service.name}
                      name={service.name}
                      status={service.status}
                      description={service.description}
                    />
                  ))}
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </motion.div>

      {/* System info */}
      <motion.div variants={itemVariants}>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">System Information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <p className="text-sm text-muted-foreground">Load Average (1m)</p>
                <p className="font-semibold">{toFixedSafe(metrics?.load1, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Load Average (5m)</p>
                <p className="font-semibold">{toFixedSafe(metrics?.load5, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Load Average (15m)</p>
                <p className="font-semibold">{toFixedSafe(metrics?.load15, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">CPU Temperature</p>
                <p className="font-semibold">
                  {metrics?.tempCpu !== undefined ? `${toFixedSafe(metrics.tempCpu, 1, '0.0')}°C` : 'N/A'}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </motion.div>
    </motion.div>
  )
}