import { useServices } from '@/api/system'
import { getErrorMessage } from '@/lib/nos-client'
import { motion } from 'framer-motion'
import { useState, useMemo, useEffect, useRef } from 'react'
import { CpuIcon, DiskIcon, FailedIcon, MemoryIcon, NetworkIcon, RefreshIcon, RisingIcon } from '@/components/icons'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { StatusMark, Working, type MarkState } from '@/components/ui/pixel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useMonitoringData, formatBytes, formatUptime } from '@/hooks/use-health'
import { useQueryClient } from '@tanstack/react-query'
import { toFixedSafe } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { chartAxis, chartGrid, chartMargin, chartTooltip, seriesLine, SeriesKey } from '@/components/ui/chart'

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
  icon: _icon,
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
    // A cell of the ruled row: its word, then its number, no icon.
    <div className="flex items-center justify-between p-3">
      <span className="text-sm text-ink-2">{label}</span>
      <motion.div
        key={value}
        initial={false}
        animate={{ opacity: 1, y: 0 }}
        className="flex items-center gap-1"
      >
        <span className={cn("font-mono tabular-nums", color)}>{value}</span>
        {unit && <span className="text-sm text-muted-foreground">{unit}</span>}
        {trend === 'up' && <RisingIcon className="h-3 w-3 text-ink-2" />}
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
  const statusConfig: Record<string, { mark: MarkState; label: string }> = {
    running: { mark: 'ok', label: 'Running' },
    stopped: { mark: 'off', label: 'Stopped' },
    error: { mark: 'failed', label: 'Error' },
    unknown: { mark: 'off', label: 'Unknown' },
  }

  const config = statusConfig[status] || statusConfig.unknown

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex justify-between items-start">
          <div>
            <CardTitle className="text-sm font-medium">{name}</CardTitle>
            <CardDescription className="text-xs mt-1">{description}</CardDescription>
          </div>
          <StatusMark state={config.mark} className="text-sm">{config.label}</StatusMark>
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
  const latest = chartData[chartData.length - 1]
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
          <h2 className="text-xl font-[560]">System monitoring</h2>
        </div>
        <Working>Reading the metrics</Working>
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
      initial={false}
      animate="visible"
      className="space-y-6"
    >
      {/* Header */}
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-[560]">System monitoring</h2>
          <StatusMark state="running" className="text-sm">Live</StatusMark>
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
            <CardTitle className="text-base">Live metrics</CardTitle>
            <CardDescription>Updated every second</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-px border border-hairline bg-hairline grid-cols-2 md:grid-cols-4 [&>*]:bg-page">
              <LiveMetric
                label="CPU"
                value={`${Math.round(metrics?.cpu || 0)}%`}
                icon={CpuIcon}
                color={metrics?.cpu && metrics.cpu > 80 ? 'text-status-failed-text' : ''}
              />
              <LiveMetric
                label="Memory"
                value={`${Math.round(metrics?.memory?.usagePct || 0)}%`}
                icon={MemoryIcon}
                color={metrics?.memory?.usagePct && metrics.memory.usagePct > 80 ? 'text-status-failed-text' : ''}
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
            <TabsTrigger value="cpu">CPU & memory</TabsTrigger>
            <TabsTrigger value="network">Network</TabsTrigger>
            <TabsTrigger value="disk">Disk I/O</TabsTrigger>
            <TabsTrigger value="services">Services</TabsTrigger>
          </TabsList>

          <TabsContent value="cpu" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">CPU & memory usage</CardTitle>
                <CardDescription>Last 60 seconds</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={chartData} margin={chartMargin}>
                    <CartesianGrid {...chartGrid} />
                    <XAxis dataKey="time" interval="preserveStartEnd" {...chartAxis} />
                    <YAxis domain={[0, 100]} tickFormatter={(value) => `${toFixedSafe(value, 0, '0')}%`} {...chartAxis} />
                    <Tooltip {...chartTooltip} formatter={(value) => `${toFixedSafe(Number(value), 1, '0.0')}%`} />
                    <Line dataKey="cpu" name="CPU" {...seriesLine(1)} />
                    <Line dataKey="memory" name="Memory" {...seriesLine(2)} />
                  </LineChart>
                </ResponsiveContainer>
                <SeriesKey
                  items={[
                    { n: 1, name: 'CPU', value: latest ? `${toFixedSafe(latest.cpu, 1, '0.0')}%` : '-' },
                    { n: 2, name: 'Memory', value: latest ? `${toFixedSafe(latest.memory, 1, '0.0')}%` : '-' },
                  ]}
                />
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="network" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Network traffic</CardTitle>
                <CardDescription>Upload and download speeds</CardDescription>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={chartData} margin={chartMargin}>
                    <CartesianGrid {...chartGrid} />
                    <XAxis dataKey="time" interval="preserveStartEnd" {...chartAxis} />
                    <YAxis tickFormatter={(value) => formatBytes(value) + '/s'} {...chartAxis} />
                    <Tooltip {...chartTooltip} formatter={(value) => formatBytes(Number(value)) + '/s'} />
                    <Line dataKey="network_rx" name="Download" {...seriesLine(1)} />
                    <Line dataKey="network_tx" name="Upload" {...seriesLine(2)} />
                  </LineChart>
                </ResponsiveContainer>
                <SeriesKey
                  items={[
                    { n: 1, name: 'Download', value: latest ? `${formatBytes(latest.network_rx)}/s` : '-' },
                    { n: 2, name: 'Upload', value: latest ? `${formatBytes(latest.network_tx)}/s` : '-' },
                  ]}
                />
                
                <div className="mt-4 grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Total downloaded</p>
                    <p className="font-mono text-lg tabular-nums">{formatBytes(stats?.totalNetworkRx || 0)}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Total uploaded</p>
                    <p className="font-mono text-lg tabular-nums">{formatBytes(stats?.totalNetworkTx || 0)}</p>
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
                  <LineChart data={chartData} margin={chartMargin}>
                    <CartesianGrid {...chartGrid} />
                    <XAxis dataKey="time" interval="preserveStartEnd" {...chartAxis} />
                    <YAxis tickFormatter={(value) => formatBytes(value) + '/s'} {...chartAxis} />
                    <Tooltip {...chartTooltip} formatter={(value) => formatBytes(Number(value)) + '/s'} />
                    <Line dataKey="disk_read" name="Read" {...seriesLine(1)} />
                    <Line dataKey="disk_write" name="Write" {...seriesLine(2)} />
                  </LineChart>
                </ResponsiveContainer>
                <SeriesKey
                  items={[
                    { n: 1, name: 'Read', value: latest ? `${formatBytes(latest.disk_read)}/s` : '-' },
                    { n: 2, name: 'Write', value: latest ? `${formatBytes(latest.disk_write)}/s` : '-' },
                  ]}
                />
                
                <div className="mt-4 grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Total read</p>
                    <p className="font-mono text-lg tabular-nums">{formatBytes(stats?.totalDiskRead || 0)}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Total written</p>
                    <p className="font-mono text-lg tabular-nums">{formatBytes(stats?.totalDiskWrite || 0)}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="services" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">System services</CardTitle>
                <CardDescription>Status of core services</CardDescription>
              </CardHeader>
              <CardContent>
                {servicesError && (
                  <p className="text-sm text-status-failed-text">The service states could not be read: {getErrorMessage(servicesError)}</p>
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
            <CardTitle className="text-base">System information</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <p className="text-sm text-muted-foreground">Load average (1m)</p>
                <p className="font-[560]">{toFixedSafe(metrics?.load1, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Load average (5m)</p>
                <p className="font-[560]">{toFixedSafe(metrics?.load5, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Load average (15m)</p>
                <p className="font-[560]">{toFixedSafe(metrics?.load15, 2, '0.00')}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">CPU temperature</p>
                <p className="font-[560]">
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