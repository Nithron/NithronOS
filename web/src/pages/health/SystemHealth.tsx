import { motion } from 'framer-motion'
import { ConnectionIcon, CpuIcon, DiskIcon, FailedIcon, FallingIcon, HealthIcon, MemoryIcon, NetworkIcon, QuickIcon, RefreshIcon, RisingIcon, TemperatureIcon, TimeIcon } from '@/components/icons'
import { useMemo, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Meter, StatusMark, Working, type MarkState } from '@/components/ui/pixel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useSystemHealth, formatBytes, formatUptime, formatPercent, getHealthStatus } from '@/hooks/use-health'
import { useQueryClient } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import { toFixedSafe, pctSafe } from '@/lib/format'

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

// Metric card component with smooth updates
function MetricCard({ 
  title, 
  value, 
  unit, 
  icon: _icon,
  trend, 
  color = 'text-primary',
  className 
}: {
  title: string
  value: string | number
  unit?: string
  icon: React.ElementType
  trend?: 'up' | 'down' | 'stable' | string
  color?: string
  className?: string
}) {
  const TrendIcon = trend === 'up' ? RisingIcon : trend === 'down' ? FallingIcon : null

  return (
    <Card className={cn("relative h-full overflow-hidden", className)}>
      <CardHeader className="pb-2">
        {/* A label is words only; the trend is an icon with its word. */}
        <CardTitle className="text-sm font-normal text-ink-2">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-baseline gap-1">
          <motion.span 
            key={value}
            initial={false}
            animate={{ opacity: 1, y: 0 }}
            className={cn('pixel-display pixel-24', color)}
          >
            {value}
          </motion.span>
          {unit && <span className="text-sm text-ink-2">{unit}</span>}
        </div>
        {TrendIcon && (
          <p className="mt-2 flex items-center gap-1 text-sm text-ink-2">
            <TrendIcon className="h-4 w-4" />
            {trend === 'up' ? 'rising' : 'falling'}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

// Progress metric component
function ProgressMetric({ 
  label, 
  value, 
  max, 
  unit = '%'
}: {
  label: string
  value: number
  max: number
  unit?: string
}) {
  const percentage = (value / max) * 100

  return (
    <div className="space-y-2">
      <div className="flex justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <motion.span 
          key={value}
          initial={false}
          animate={{ opacity: 1 }}
          className="font-medium"
        >
          {formatPercent(percentage)}{unit === '%' ? '' : ` (${formatBytes(value)} / ${formatBytes(max)})`}
        </motion.span>
      </div>
      <Meter
        value={percentage}
        blocks={20}
        label={label}
        showValue={false}
        tone={percentage > 90 ? 'failed' : percentage > 80 ? 'warning' : 'ink'}
      />
    </div>
  )
}

export default function SystemHealth() {
  const queryClient = useQueryClient()
  const { data: health, isLoading, error, refetch } = useSystemHealth()
  const [isManualRefreshing, setIsManualRefreshing] = useState(false)

  // Calculate derived metrics
  const metrics = useMemo(() => {
    if (!health) return null

    const cpuTrend = health.cpu > 50 ? 'up' : health.cpu < 20 ? 'down' : 'stable'
    const memoryTrend = health.memory.usagePct > 70 ? 'up' : 'stable'
    const overallStatus = getHealthStatus(
      health.cpu,
      health.memory.usagePct,
      0 // We'll get disk usage from disk health
    )

    return {
      cpu: health.cpu,
      cpuTrend,
      memory: health.memory,
      memoryTrend,
      swap: health.swap,
      uptime: health.uptimeSec,
      load: {
        load1: health.load1,
        load5: health.load5,
        load15: health.load15
      },
      network: health.network,
      diskIO: health.diskIO,
      tempCpu: health.tempCpu,
      overallStatus
    }
  }, [health])

  const handleManualRefresh = async () => {
    setIsManualRefreshing(true)
    await queryClient.invalidateQueries({ queryKey: ['health', 'system'] })
    await refetch()
    setTimeout(() => setIsManualRefreshing(false), 1000)
  }

  if (isLoading && !health) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h2 className="text-xl font-[560]">System health</h2>
        </div>
        <Working>Reading the system</Working>
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

  const overallMark: MarkState =
    metrics?.overallStatus === 'critical' ? 'failed' : metrics?.overallStatus === 'warning' ? 'warning' : metrics?.overallStatus ? 'ok' : 'off'

  return (
    <motion.div
      variants={containerVariants}
      initial={false}
      animate="visible"
      className="space-y-6"
    >
      {/* Header with status */}
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-[560]">System health</h2>
          <StatusMark state={overallMark} className="text-sm">{metrics?.overallStatus || 'unknown'}</StatusMark>
        </div>
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

      {/* Quick metrics grid */}
      <motion.div 
        variants={containerVariants}
        className="grid border-l border-t border-hairline md:grid-cols-2 lg:grid-cols-4 [&>*]:border-b [&>*]:border-r [&>*]:border-hairline [&>*>*]:h-full [&>*>*]:border-0"
      >
        <motion.div variants={itemVariants}>
          <MetricCard
            title="CPU usage"
            value={pctSafe(metrics?.cpu, 0)}
            icon={CpuIcon}
            trend={metrics?.cpuTrend}
            color={metrics?.cpu && metrics.cpu > 80 ? 'text-status-failed-text' : 'text-primary'}
          />
        </motion.div>
        
        <motion.div variants={itemVariants}>
          <MetricCard
            title="Memory"
            value={pctSafe(metrics?.memory?.usagePct, 0)}
            icon={MemoryIcon}
            trend={metrics?.memoryTrend}
            color={metrics?.memory?.usagePct && metrics.memory.usagePct > 80 ? 'text-status-failed-text' : 'text-primary'}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="System load"
            value={toFixedSafe(metrics?.load?.load1, 2, '0.00')}
            icon={HealthIcon}
            trend={metrics?.load?.load1 && metrics.load.load1 > metrics.load.load5 ? 'up' : 'down'}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="Uptime"
            value={formatUptime(metrics?.uptime || 0)}
            icon={TimeIcon}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="Network RX"
            value={formatBytes(metrics?.network?.rxSpeed || 0)}
            unit="/s"
            icon={NetworkIcon}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="Network TX"
            value={formatBytes(metrics?.network?.txSpeed || 0)}
            unit="/s"
            icon={ConnectionIcon}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="Disk read"
            value={formatBytes(metrics?.diskIO?.readSpeed || 0)}
            unit="/s"
            icon={DiskIcon}
          />
        </motion.div>

        <motion.div variants={itemVariants}>
          <MetricCard
            title="Disk write"
            value={formatBytes(metrics?.diskIO?.writeSpeed || 0)}
            unit="/s"
            icon={QuickIcon}
          />
        </motion.div>
      </motion.div>

      {/* Detailed metrics tabs */}
      <motion.div variants={itemVariants}>
        <Tabs defaultValue="resources" className="space-y-4">
          <TabsList>
            <TabsTrigger value="resources">Resources</TabsTrigger>
            <TabsTrigger value="network">Network</TabsTrigger>
            <TabsTrigger value="system">System</TabsTrigger>
          </TabsList>

          <TabsContent value="resources" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Resource usage</CardTitle>
                <CardDescription>CPU, Memory, and Swap utilization</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <ProgressMetric
                  label="CPU"
                  value={metrics?.cpu || 0}
                  max={100}
                />
                <ProgressMetric
                  label="Memory"
                  value={metrics?.memory?.used || 0}
                  max={metrics?.memory?.total || 1}
                  unit="bytes"
                />
                {metrics?.swap?.total ? (
                  <ProgressMetric
                    label="Swap"
                    value={metrics?.swap?.used || 0}
                    max={metrics?.swap?.total || 1}
                    unit="bytes"
                  />
                ) : null}
                <div className="grid grid-cols-3 gap-4 pt-4 border-t">
                  <div>
                    <p className="text-sm text-muted-foreground">Load (1m)</p>
                    <p className="text-lg font-[560]">{toFixedSafe(metrics?.load?.load1, 2, '0.00')}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Load (5m)</p>
                    <p className="text-lg font-[560]">{toFixedSafe(metrics?.load?.load5, 2, '0.00')}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Load (15m)</p>
                    <p className="text-lg font-[560]">{toFixedSafe(metrics?.load?.load15, 2, '0.00')}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="network" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Network statistics</CardTitle>
                <CardDescription>Network interface traffic and speeds</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <p className="text-sm text-muted-foreground">Download speed</p>
                    <p className="text-2xl font-[560]">{formatBytes(metrics?.network?.rxSpeed || 0)}/s</p>
                    <p className="text-xs text-muted-foreground">
                      Total: {formatBytes(metrics?.network?.bytesRecv || 0)}
                    </p>
                  </div>
                  <div className="space-y-2">
                    <p className="text-sm text-muted-foreground">Upload speed</p>
                    <p className="text-2xl font-[560]">{formatBytes(metrics?.network?.txSpeed || 0)}/s</p>
                    <p className="text-xs text-muted-foreground">
                      Total: {formatBytes(metrics?.network?.bytesSent || 0)}
                    </p>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4 pt-4 border-t">
                  <div>
                    <p className="text-sm text-muted-foreground">Packets received</p>
                    <p className="text-lg font-[560]">{metrics?.network?.packetsRecv?.toLocaleString() || '0'}</p>
                  </div>
                  <div>
                    <p className="text-sm text-muted-foreground">Packets sent</p>
                    <p className="text-lg font-[560]">{metrics?.network?.packetsSent?.toLocaleString() || '0'}</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="system" className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">System information</CardTitle>
                <CardDescription>Temperature, uptime, and system details</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm text-muted-foreground">Uptime</p>
                    <p className="text-lg font-[560]">{formatUptime(metrics?.uptime || 0)}</p>
                  </div>
                  {metrics?.tempCpu !== undefined && (
                    <div>
                      <p className="text-sm text-muted-foreground">CPU temperature</p>
                      <p className="text-lg font-[560] flex items-center gap-1">
                        <TemperatureIcon className="h-4 w-4" />
                        {metrics?.tempCpu !== undefined ? `${toFixedSafe(metrics.tempCpu, 1, '0.0')}°C` : 'N/A'}
                      </p>
                    </div>
                  )}
                </div>
                <div className="space-y-2 pt-4 border-t">
                  <h4 className="text-sm font-medium">Disk I/O</h4>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <p className="text-sm text-muted-foreground">Read speed</p>
                      <p className="text-lg font-[560]">{formatBytes(metrics?.diskIO?.readSpeed || 0)}/s</p>
                      <p className="text-xs text-muted-foreground">
                        Total: {formatBytes(metrics?.diskIO?.readBytes || 0)}
                      </p>
                    </div>
                    <div className="space-y-1">
                      <p className="text-sm text-muted-foreground">Write speed</p>
                      <p className="text-lg font-[560]">{formatBytes(metrics?.diskIO?.writeSpeed || 0)}/s</p>
                      <p className="text-xs text-muted-foreground">
                        Total: {formatBytes(metrics?.diskIO?.writeBytes || 0)}
                      </p>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </motion.div>
    </motion.div>
  )
}