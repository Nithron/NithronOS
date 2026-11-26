import { useState } from 'react'
import { 
  HardDrive, 
  Database,
  Activity,
  Plus,
  RefreshCw,
  ChevronRight,
  Thermometer,
  Clock,
} from 'lucide-react'
import { ColumnDef } from '@tanstack/react-table'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { EmptyState } from '@/components/ui/empty-state'
import { StatusPill } from '@/components/ui/status'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/ui/data-table'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import {
  usePools,
  useDevices,
  useSmartDevice,
  useScrubStatus,
  useBalanceStatus,
  useMaintenanceMutations,
  type Device,
  type Pool,
} from '@/api/storage'
import { cn } from '@/lib/utils'
import { getErrorMessage } from '@/lib/nos-client'
import { toast } from '@/components/ui/toast'

// A device's kind: rotational disks are HDDs.
function deviceKind(d: Device): string {
  if (d.rota === undefined) return 'Unknown'
  return d.rota ? 'HDD' : 'SSD'
}

// What a device already holds, if anything.
function deviceUse(d: Device): string | null {
  if (d.btrfsMember) return 'Btrfs pool member'
  if (d.luks) return 'Encrypted (LUKS)'
  if (d.fsType) return d.fsType
  return null
}

// Helper functions
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

// Device columns
const deviceColumns: ColumnDef<Device>[] = [
  {
    accessorKey: 'path',
    header: 'Device',
    cell: ({ row }) => (
      <div className="flex items-center gap-2">
        <HardDrive className="h-4 w-4 text-muted-foreground" />
        <span className="font-mono">{row.original.path}</span>
      </div>
    ),
  },
  {
    accessorKey: 'model',
    header: 'Model',
    cell: ({ row }) => row.original.model || 'Unknown',
  },
  {
    accessorKey: 'size',
    header: 'Size',
    cell: ({ row }) => formatBytes(row.original.size),
  },
  {
    accessorKey: 'type',
    header: 'Type',
    cell: ({ row }) => (
      <span className="uppercase text-xs font-medium">
        {deviceKind(row.original)}
      </span>
    ),
  },
  {
    accessorKey: 'fsType',
    header: 'Status',
    cell: ({ row }) => {
      const use = deviceUse(row.original)
      return (
        <StatusPill status={use ? 'active' : 'inactive'} size="sm">
          {use ? `In use: ${use}` : 'Available'}
        </StatusPill>
      )
    },
  },
  {
    accessorKey: 'actions',
    header: '',
    cell: ({ row }) => (
      <Button 
        variant="ghost" 
        size="icon"
        disabled={deviceUse(row.original) !== null}
      >
        <ChevronRight className="h-4 w-4" />
      </Button>
    ),
  },
]

// Pool columns
const poolColumns: ColumnDef<Pool>[] = [
  {
    accessorKey: 'label',
    header: 'Pool',
    cell: ({ row }) => (
      <div className="flex items-center gap-2">
        <Database className="h-4 w-4 text-muted-foreground" />
        <div>
          <div className="font-medium">{row.original.label || row.original.id}</div>
          <div className="text-xs text-muted-foreground">{row.original.mount ?? 'Not mounted'}</div>
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'raid',
    header: 'RAID',
    cell: ({ row }) => (
      <span className="uppercase text-xs font-medium">
        {row.original.raid}
      </span>
    ),
  },
  {
    accessorKey: 'size',
    header: 'Capacity',
    cell: ({ row }) => {
      const { size, used } = row.original
      const percentage = size > 0 ? (used / size) * 100 : 0
      return (
        <div className="space-y-1">
          <div className="text-sm">
            {formatBytes(used)} / {formatBytes(size)}
          </div>
          <div className="h-2 bg-muted rounded-full overflow-hidden w-24">
            <div 
              className={cn(
                "h-full transition-all",
                percentage < 70 ? "bg-green-500" : 
                percentage < 90 ? "bg-yellow-500" : "bg-red-500"
              )}
              style={{ width: `${percentage}%` }}
            />
          </div>
        </div>
      )
    },
  },
  {
    accessorKey: 'mount',
    header: 'Status',
    cell: ({ row }) => (
      <StatusPill status={row.original.mount ? 'healthy' : 'inactive'} size="sm">
        {row.original.mount ? 'Mounted' : 'Not mounted'}
      </StatusPill>
    ),
  },
  {
    accessorKey: 'devices',
    header: 'Devices',
    cell: ({ row }) => (
      <span className="text-sm">
        {row.original.devices.length} device{row.original.devices.length !== 1 ? 's' : ''}
      </span>
    ),
  },
  {
    accessorKey: 'actions',
    header: '',
    cell: ({ row }) => (
      <Button 
        variant="ghost" 
        size="icon"
        onClick={() => window.location.href = `/storage/${row.original.id}`}
      >
        <ChevronRight className="h-4 w-4" />
      </Button>
    ),
  },
]

export function Storage() {
  const [activeTab, setActiveTab] = useState<'pools' | 'devices'>('pools')
  const [selectedDevice, setSelectedDevice] = useState<string | null>(null)
  const [isRefreshing, setIsRefreshing] = useState(false)
  
  // Fetch data from API
  const { data: pools, isLoading: poolsLoading, error: poolsError, refetch: refetchPools } = usePools()
  const { data: devices, isLoading: devicesLoading, error: devicesError, refetch: refetchDevices } = useDevices()
  const { data: scrubStatus, refetch: refetchScrub } = useScrubStatus()
  const { data: balanceStatus, refetch: refetchBalance } = useBalanceStatus()
  const { data: smartData, error: smartError } = useSmartDevice(selectedDevice)

  // Mutations
  const { startScrub, startBalance } = useMaintenanceMutations()

  const handleRefresh = async () => {
    setIsRefreshing(true)
    await Promise.all([
      refetchPools(),
      refetchDevices(),
      refetchScrub(),
      refetchBalance(),
    ])
    setTimeout(() => setIsRefreshing(false), 500)
  }

  // Each pool's button starts that pool (an empty request used to start
  // every pool).
  const handleStartScrub = (poolId: string) =>
    startScrub.mutate(poolId, {
      onSuccess: () => toast.success('Scrub started'),
      onError: (e) => toast.error(getErrorMessage(e)),
    })

  const handleStartBalance = (poolId: string) =>
    startBalance.mutate(poolId, {
      onSuccess: () => toast.success('Balance started'),
      onError: (e) => toast.error(getErrorMessage(e)),
    })

  return (
    <div className="space-y-6">
      <PageHeader
        title="Storage"
        description="Manage storage pools and devices"
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
              Refresh
            </Button>
            <Button 
              size="sm"
              onClick={() => window.location.href = '/storage/create'}
            >
              <Plus className="h-4 w-4 mr-2" />
              Create Pool
            </Button>
          </>
        }
      />

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as any)}>
        <TabsList>
          <TabsTrigger value="pools">Storage Pools</TabsTrigger>
          <TabsTrigger value="devices">Devices</TabsTrigger>
        </TabsList>

        <TabsContent value="pools" className="space-y-6">
          {/* Pool Summary Cards */}
          {pools && pools.length > 0 && (
            <div className="grid gap-4 md:grid-cols-3">
              <Card className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-muted-foreground">Total Capacity</p>
                    <p className="text-2xl font-bold">
                      {formatBytes(pools.reduce((acc, p) => acc + p.size, 0))}
                    </p>
                  </div>
                  <Database className="h-8 w-8 text-muted-foreground" />
                </div>
              </Card>
              
              <Card className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-muted-foreground">Used Space</p>
                    <p className="text-2xl font-bold">
                      {formatBytes(pools.reduce((acc, p) => acc + p.used, 0))}
                    </p>
                  </div>
                  <HardDrive className="h-8 w-8 text-muted-foreground" />
                </div>
              </Card>
              
              <Card className="p-6">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm text-muted-foreground">Pools mounted</p>
                    <div className="flex items-center gap-2 mt-1">
                      <span className="text-2xl font-bold">
                        {pools.filter(p => p.mount).length}/{pools.length}
                      </span>
                      <span className="text-sm text-muted-foreground">mounted</span>
                    </div>
                  </div>
                  <Activity className="h-8 w-8 text-muted-foreground" />
                </div>
              </Card>
            </div>
          )}

          {/* Pools Table */}
          <Card
            title="Storage Pools"
            description={`${pools?.length || 0} configured`}
            isLoading={poolsLoading}
            error={poolsError ? getErrorMessage(poolsError) : null}
            onRetry={() => refetchPools()}
          >
            {pools && pools.length > 0 ? (
              <DataTable 
                columns={poolColumns} 
                data={pools}
                searchKey="label"
                searchPlaceholder="Search pools..."
              />
            ) : (
              <EmptyState
                icon={Database}
                title="No storage pools"
                description="Create your first storage pool to start storing data"
                action={
                  <Button onClick={() => window.location.href = '/storage/create'}>
                    <Plus className="h-4 w-4 mr-2" />
                    Create Pool
                  </Button>
                }
              />
            )}
          </Card>

          {/* Maintenance Operations */}
          {pools && pools.length > 0 && (
            <div className="grid gap-6 md:grid-cols-2">
              {/* Scrub Status */}
              <Card
                title="Data Scrub"
                description="Verify data integrity"
                actions={
                  pools[0] && (
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => handleStartScrub(pools[0].id)}
                      disabled={scrubStatus?.some(s => s.status === 'running')}
                    >
                      Start Scrub
                    </Button>
                  )
                }
              >
                {scrubStatus && scrubStatus.length > 0 ? (
                  <div className="space-y-3">
                    {scrubStatus.map((scrub) => (
                      <div key={scrub.poolId} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium">{scrub.poolId}</span>
                          <StatusPill 
                            status={scrub.status === 'running' ? 'running' : 
                                   scrub.status === 'finished' ? 'completed' : 'idle'} 
                            size="sm" 
                          />
                        </div>
                        {scrub.progress !== undefined && scrub.status === 'running' && (
                          <div className="space-y-1">
                            <div className="flex justify-between text-xs text-muted-foreground">
                              <span>Progress</span>
                              <span>{scrub.progress}%</span>
                            </div>
                            <div className="h-2 bg-muted rounded-full overflow-hidden">
                              <div 
                                className="h-full bg-primary transition-all"
                                style={{ width: `${scrub.progress}%` }}
                              />
                            </div>
                          </div>
                        )}
                        {scrub.status === 'finished' && scrub.finishedAt && (
                          <p className="text-xs text-muted-foreground">
                            Last finished {new Date(scrub.finishedAt).toLocaleString()}
                            {scrub.errors ? `, ${scrub.errors} errors` : ', no errors'}
                          </p>
                        )}
                        {scrub.error && <p className="text-xs text-destructive">{scrub.error}</p>}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No scrub operations scheduled
                  </p>
                )}
              </Card>

              {/* Balance Status */}
              <Card
                title="Data Balance"
                description="Redistribute data across devices"
                actions={
                  pools[0] && (
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => handleStartBalance(pools[0].id)}
                      disabled={balanceStatus?.some(b => b.status === 'running')}
                    >
                      Start Balance
                    </Button>
                  )
                }
              >
                {balanceStatus && balanceStatus.length > 0 ? (
                  <div className="space-y-3">
                    {balanceStatus.map((balance) => (
                      <div key={balance.poolId} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium">{balance.poolId}</span>
                          <StatusPill 
                            status={balance.status === 'running' ? 'running' : 'idle'} 
                            size="sm" 
                          />
                        </div>
                        {balance.progress !== undefined && balance.status === 'running' && (
                          <div className="space-y-1">
                            <div className="flex justify-between text-xs text-muted-foreground">
                              <span>Progress</span>
                              <span>{balance.progress}%</span>
                            </div>
                            <div className="h-2 bg-muted rounded-full overflow-hidden">
                              <div 
                                className="h-full bg-primary transition-all"
                                style={{ width: `${balance.progress}%` }}
                              />
                            </div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No balance operations in progress
                  </p>
                )}
              </Card>
            </div>
          )}
        </TabsContent>

        <TabsContent value="devices" className="space-y-6">
          {/* Devices Table */}
          <Card
            title="Storage Devices"
            description={`${devices?.length || 0} detected`}
            isLoading={devicesLoading}
            error={devicesError ? getErrorMessage(devicesError) : null}
            onRetry={() => refetchDevices()}
          >
            {devices && devices.length > 0 ? (
              <DataTable 
                columns={deviceColumns} 
                data={devices}
                searchKey="path"
                searchPlaceholder="Search devices..."
                onRowClick={(device) => setSelectedDevice(device.name)}
              />
            ) : (
              <EmptyState
                icon={HardDrive}
                title="No devices found"
                description="No storage devices were detected in the system"
              />
            )}
          </Card>

          {/* SMART Details */}
          {selectedDevice && (
            <Card
              title="SMART Details"
              description={selectedDevice}
            >
              {smartError && <p className="text-sm text-destructive">{getErrorMessage(smartError)}</p>}
              {smartData ? (
                <div className="space-y-4">
                  {/* SMART Status */}
                  <div className="flex items-center justify-between p-4 bg-muted/50 rounded-lg">
                    <div className="flex items-center gap-3">
                      <div className={cn(
                        "h-3 w-3 rounded-full",
                        smartData.health === 'good' && "bg-green-500",
                        smartData.health === 'warning' && "bg-yellow-500",
                        smartData.health === 'critical' && "bg-red-500",
                        smartData.health === 'unknown' && "bg-muted-foreground"
                      )} />
                      <div>
                        <p className="font-medium">Overall Health</p>
                        <p className="text-sm text-muted-foreground">
                          {smartData.health === 'unknown' ? 'No SMART data' : smartData.health}
                        </p>
                      </div>
                    </div>
                    <div className="text-right">
                      {smartData.temperature_celsius !== undefined && (
                        <div className="flex items-center gap-2">
                          <Thermometer className="h-4 w-4 text-muted-foreground" />
                          <span>{smartData.temperature_celsius}°C</span>
                        </div>
                      )}
                      {smartData.power_on_hours !== undefined && (
                        <div className="flex items-center gap-2 mt-1">
                          <Clock className="h-4 w-4 text-muted-foreground" />
                          <span className="text-sm text-muted-foreground">
                            {smartData.power_on_hours} hours
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">
                  Select a device to view SMART details
                </p>
              )}
            </Card>
          )}
        </TabsContent>
      </Tabs>
    </div>
  )
}