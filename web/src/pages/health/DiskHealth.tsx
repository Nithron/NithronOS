import { motion } from 'framer-motion'
import { FailedIcon, NoticeIcon, RefreshIcon, StartIcon, TemperatureIcon, WorkingIcon } from '@/components/icons'
import { useMemo, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Meter, StatusMark, Working, type MarkState } from '@/components/ui/pixel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useDiskHealth, formatBytes } from '@/hooks/use-health'
import { useQueryClient } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import { toFixedSafe } from '@/lib/format'
import type { DiskHealth } from '@/lib/api-health'

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

// Health status component
const diskMark = (state: string): MarkState =>
  state === 'critical' ? 'failed' : state === 'warning' ? 'warning' : state === 'healthy' ? 'ok' : 'off'

function HealthStatus({ state }: { state: string }) {
  return <StatusMark state={diskMark(state)} className="text-sm">{state}</StatusMark>
}

// A temperature as a state: over 50 °C failing, over 40 °C a warning.
/** A temperature the disk reported; 0 means none was read. */
const hasTemp = (c?: number): c is number => typeof c === 'number' && c > 0

const tempMark = (c: number): MarkState => (c > 50 ? 'failed' : c > 40 ? 'warning' : 'ok')

// Disk card component
function DiskCard({ 
  disk, 
  isSelected, 
  onClick 
}: { 
  disk: DiskHealth
  isSelected: boolean
  onClick: () => void 
}) {
  return (
    <motion.div variants={itemVariants}>
      <Card
        role="button"
        tabIndex={0}
        aria-pressed={isSelected}
        className={cn('cursor-pointer hover:bg-ink/[.09] focus-visible:outline focus-visible:outline-2 focus-visible:outline-ink', isSelected && 'outline outline-2 -outline-offset-2 outline-ink')}
        onClick={onClick}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onClick()
          }
        }}
      >
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 space-y-1">
              <CardTitle className="break-all font-mono text-sm font-normal">
                {disk.name}
              </CardTitle>
              <CardDescription className="text-xs">
                {disk.model || 'Unknown model'}, <span className="font-mono">{formatBytes(disk.sizeBytes)}</span>
              </CardDescription>
            </div>
            <HealthStatus state={disk.state} />
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <p className="text-sm text-ink-2">Usage</p>
            <Meter
              value={disk.usagePct}
              blocks={16}
              label={`${disk.name} used`}
              tone={disk.usagePct > 90 ? 'failed' : disk.usagePct > 80 ? 'warning' : 'ink'}
            />
          </div>
          
          {hasTemp(disk.tempC) && (
            <div className="flex justify-between items-center text-sm">
              <span className="text-muted-foreground flex items-center gap-1">
                <TemperatureIcon className="h-3 w-3" />
                Temperature
              </span>
              <StatusMark state={tempMark(disk.tempC)} className="font-mono tabular-nums">
                {toFixedSafe(disk.tempC, 0, '0')} °C
              </StatusMark>
            </div>
          )}

          <div className="flex justify-between items-center text-sm">
            <span className="text-muted-foreground">SMART</span>
            <StatusMark state={disk.smart.passed ? 'ok' : 'failed'}>
              {disk.smart.passed ? 'Passed' : 'Failed'}
            </StatusMark>
          </div>

          <div className="border-t border-hairline pt-2">
            <div className="grid grid-cols-2 gap-2 text-xs">
              <div>
                <p className="text-muted-foreground">Filesystem</p>
                <p className="font-medium">{disk.filesystem || 'Unknown'}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Mount</p>
                <p className="font-medium truncate" title={disk.mountPoint}>
                  {disk.mountPoint || 'Not mounted'}
                </p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </motion.div>
  )
}

export default function DiskHealth() {
  const queryClient = useQueryClient()
  const { data: rawDisks, isLoading, error, refetch } = useDiskHealth()
  const [selectedDisk, setSelectedDisk] = useState<string | null>(null)
  const [isManualRefreshing, setIsManualRefreshing] = useState(false)
  const [runningTests, setRunningTests] = useState<Set<string>>(new Set())

  // Ensure disks is always an array
  const disks = useMemo(() => {
    if (!rawDisks) return []
    return Array.isArray(rawDisks) ? rawDisks : []
  }, [rawDisks])

  // Calculate summary statistics
  const summary = useMemo(() => {
    if (!disks || disks.length === 0) return null

    const totalCapacity = disks.reduce((sum, disk) => sum + disk.sizeBytes, 0)
    const totalUsed = disks.reduce((sum, disk) => sum + (disk.sizeBytes * disk.usagePct / 100), 0)
    const healthyDisks = disks.filter(d => d.state === 'healthy').length
    const warningDisks = disks.filter(d => d.state === 'warning').length
    const criticalDisks = disks.filter(d => d.state === 'critical').length
    const measured = disks.filter((d) => hasTemp(d.tempC))
    const avgTemp = measured.reduce((sum, d) => sum + (d.tempC || 0), 0) / (measured.length || 1)

    return {
      totalDisks: disks.length,
      totalCapacity,
      totalUsed,
      usagePercent: (totalUsed / totalCapacity) * 100,
      healthyDisks,
      warningDisks,
      criticalDisks,
      avgTemp: measured.length > 0 ? avgTemp : undefined
    }
  }, [disks])

  const selectedDiskData = useMemo(() => {
    if (!selectedDisk || !disks) return null
    return disks.find((d: DiskHealth) => d.id === selectedDisk) || null
  }, [selectedDisk, disks])

  const handleManualRefresh = async () => {
    setIsManualRefreshing(true)
    await queryClient.invalidateQueries({ queryKey: ['health', 'disks'] })
    await refetch()
    setTimeout(() => setIsManualRefreshing(false), 1000)
  }

  const handleRunTest = async (diskId: string, testType: 'short' | 'long' | 'conveyance') => {
    setRunningTests(prev => new Set(prev).add(diskId))
    
    // Simulate test execution
    setTimeout(() => {
      setRunningTests(prev => {
        const next = new Set(prev)
        next.delete(diskId)
        return next
      })
    }, testType === 'short' ? 5000 : testType === 'conveyance' ? 10000 : 30000)
  }

  if (isLoading && !disks) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h2 className="text-xl font-[560]">Disk health</h2>
        </div>
        <Working>Reading the disks</Working>
      </div>
    )
  }

  if (error) {
    return (
      <div className="space-y-6">
        <Alert variant="destructive">
          <FailedIcon className="h-4 w-4" />
          <AlertDescription>
            Failed to load disk health data. 
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

  if (!disks || disks.length === 0) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h2 className="text-xl font-[560]">Disk health</h2>
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
        <Alert>
          <NoticeIcon className="h-4 w-4" />
          <AlertDescription>
            No disks detected. Make sure disks are properly connected and the system has necessary permissions.
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
          <h2 className="text-xl font-[560]">Disk health</h2>
          {summary && (
            <span className="font-mono text-sm text-ink-2">
              {summary.totalDisks} {summary.totalDisks === 1 ? 'disk' : 'disks'}
            </span>
          )}
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

      {/* Summary stats */}
      {summary && (
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Storage overview</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div>
                  <p className="text-sm text-muted-foreground">Total capacity</p>
                  <p className="text-xl font-bold">{formatBytes(summary.totalCapacity)}</p>
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Used space</p>
                  <p className="text-xl font-bold">{formatBytes(summary.totalUsed)}</p>
                  <p className="text-xs text-muted-foreground">{Math.round(summary.usagePercent)}% used</p>
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">Disk status</p>
                  <div className="flex gap-2 mt-1">
                    {summary.healthyDisks > 0 && <StatusMark state="ok" className="text-sm">{summary.healthyDisks} healthy</StatusMark>}
                    {summary.warningDisks > 0 && <StatusMark state="warning" className="text-sm">{summary.warningDisks} warning</StatusMark>}
                    {summary.criticalDisks > 0 && <StatusMark state="failed" className="text-sm">{summary.criticalDisks} failing</StatusMark>}
                  </div>
                </div>
                {summary.avgTemp !== undefined && (
                  <div>
                    <p className="text-sm text-muted-foreground">Avg temperature</p>
                    <p className="font-mono text-xl tabular-nums">
                      {toFixedSafe(summary.avgTemp, 0, '0')} °C
                    </p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      )}

      {/* Disk cards grid */}
      <motion.div 
        variants={containerVariants}
        className="grid border-l border-t border-hairline md:grid-cols-2 lg:grid-cols-3 [&>*]:border-b [&>*]:border-r [&>*]:border-hairline [&>*>*]:h-full [&>*>*]:border-0"
      >
        {disks.map((disk) => (
          <DiskCard
            key={disk.id}
            disk={disk}
            isSelected={selectedDisk === disk.id}
            onClick={() => setSelectedDisk(disk.id === selectedDisk ? null : disk.id)}
          />
        ))}
      </motion.div>

      {/* Selected disk details */}
      {selectedDiskData && (
        <motion.div variants={itemVariants}>
          <Card>
            <CardHeader>
              <CardTitle className="text-base flex items-center gap-2">
                {selectedDiskData.name} Details
              </CardTitle>
              <CardDescription>
                Detailed information and SMART attributes
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="info" className="space-y-4">
                <TabsList>
                  <TabsTrigger value="info">Information</TabsTrigger>
                  <TabsTrigger value="smart">SMART data</TabsTrigger>
                  <TabsTrigger value="tests">Tests</TabsTrigger>
                </TabsList>

                <TabsContent value="info" className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="text-sm text-muted-foreground">Model</p>
                      <p className="font-medium">{selectedDiskData.model || 'Unknown'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Serial number</p>
                      <p className="font-medium">{selectedDiskData.serial || 'Unknown'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Capacity</p>
                      <p className="font-medium">{formatBytes(selectedDiskData.sizeBytes)}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Filesystem</p>
                      <p className="font-medium">{selectedDiskData.filesystem || 'Unknown'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Mount point</p>
                      <p className="font-medium">{selectedDiskData.mountPoint || 'Not mounted'}</p>
                    </div>
                    <div>
                      <p className="text-sm text-muted-foreground">Usage</p>
                      <p className="font-medium">{Math.round(selectedDiskData.usagePct)}%</p>
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="smart" className="space-y-4">
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium">SMART status</span>
                      <StatusMark state={selectedDiskData.smart.passed ? 'ok' : 'failed'}>
                        {selectedDiskData.smart.passed ? 'Passed' : 'Failed'}
                      </StatusMark>
                    </div>
                    
                    {selectedDiskData.smart.attrs && Object.keys(selectedDiskData.smart.attrs).length > 0 ? (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Attribute</TableHead>
                            <TableHead>Value</TableHead>
                            <TableHead>Threshold</TableHead>
                            <TableHead>Status</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {Object.entries(selectedDiskData.smart.attrs).map(([key, value]: [string, any]) => (
                            <TableRow key={key}>
                              <TableCell className="font-mono">{key}</TableCell>
                              <TableCell className="font-mono tabular-nums">{value.value || '-'}</TableCell>
                              <TableCell className="font-mono tabular-nums">{value.threshold || '-'}</TableCell>
                              <TableCell>
                                <StatusMark state={value.status === 'ok' ? 'ok' : value.status ? 'failed' : 'off'}>
                                  {value.status || 'unknown'}
                                </StatusMark>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    ) : (
                      <Alert>
                        <NoticeIcon className="h-4 w-4" />
                        <AlertDescription>
                          No SMART attributes available for this disk.
                        </AlertDescription>
                      </Alert>
                    )}
                  </div>
                </TabsContent>

                <TabsContent value="tests" className="space-y-4">
                  <div className="space-y-3">
                    <p className="text-sm text-muted-foreground">
                      Run SMART self-tests to check disk health
                    </p>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleRunTest(selectedDiskData.id, 'short')}
                        disabled={runningTests.has(selectedDiskData.id)}
                      >
                        <StartIcon className="h-3 w-3 mr-1" />
                        Short Test (~2 min)
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleRunTest(selectedDiskData.id, 'conveyance')}
                        disabled={runningTests.has(selectedDiskData.id)}
                      >
                        <StartIcon className="h-3 w-3 mr-1" />
                        Conveyance Test (~5 min)
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleRunTest(selectedDiskData.id, 'long')}
                        disabled={runningTests.has(selectedDiskData.id)}
                      >
                        <StartIcon className="h-3 w-3 mr-1" />
                        Extended Test (~30 min)
                      </Button>
                    </div>
                    {runningTests.has(selectedDiskData.id) && (
                      <Alert>
                        <WorkingIcon className="h-4 w-4" />
                        <AlertDescription>
                          Test is running... This may take several minutes.
                        </AlertDescription>
                      </Alert>
                    )}
                  </div>
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </motion.div>
      )}
    </motion.div>
  )
}