import { useState } from 'react'
import { formatDateTime } from '@/lib/datetime'
import { Link } from 'react-router-dom'
import { AddIcon, PoolIcon, RefreshIcon, WorkingIcon } from '@/components/icons'
import { ColumnDef } from '@tanstack/react-table'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { EmptyState } from '@/components/ui/empty-state'
import { Meter, StatusMark } from '@/components/ui/pixel'
import { Button, buttonVariants } from '@/components/ui/button'
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
      <span className="font-mono">{row.original.path}</span>
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
    cell: ({ row }) => <span className="font-mono tabular-nums">{formatBytes(row.original.size)}</span>,
  },
  {
    accessorKey: 'type',
    header: 'Type',
    cell: ({ row }) => (
      <span className="font-mono text-xs">{deviceKind(row.original)}</span>
    ),
  },
  {
    accessorKey: 'fsType',
    header: 'Status',
    cell: ({ row }) => {
      // In use or free is no state of health: words, no colour.
      const use = deviceUse(row.original)
      return use ? <span className="text-ink-2">In use: {use}</span> : <span>Available</span>
    },
  },
]

// Pool columns
const poolColumns: ColumnDef<Pool>[] = [
  {
    accessorKey: 'label',
    header: 'Pool',
    cell: ({ row }) => (
      <div>
        <Link to={`/storage/${encodeURIComponent(row.original.id)}`} className="font-[560] underline decoration-ink-2 underline-offset-4 hover:decoration-ink">
          {row.original.label || row.original.id}
        </Link>
        <div className="font-mono text-xs text-ink-2">{row.original.mount ?? 'Not mounted'}</div>
      </div>
    ),
  },
  {
    accessorKey: 'raid',
    header: 'RAID',
    cell: ({ row }) => (
      <span className="font-mono text-xs uppercase">
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
          <div className="font-mono text-sm tabular-nums">
            {formatBytes(used)} / {formatBytes(size)}
          </div>
          <Meter
            value={used}
            max={size}
            blocks={10}
            label={`${row.original.label || row.original.id} used`}
            valueText={`${percentage.toFixed(0)}%`}
            tone={percentage >= 90 ? 'failed' : percentage >= 80 ? 'warning' : 'ink'}
          />
        </div>
      )
    },
  },
  {
    accessorKey: 'mount',
    header: 'Status',
    cell: ({ row }) => (
      <StatusMark state={row.original.mount ? 'ok' : 'off'}>
        {row.original.mount ? 'Mounted' : 'Not mounted'}
      </StatusMark>
    ),
  },
  {
    accessorKey: 'devices',
    header: 'Devices',
    cell: ({ row }) => (
      <span className="font-mono text-sm tabular-nums">
        {row.original.devices.length} device{row.original.devices.length !== 1 ? 's' : ''}
      </span>
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

  const totalSize = (pools ?? []).reduce((acc, p) => acc + p.size, 0)
  const totalUsed = (pools ?? []).reduce((acc, p) => acc + p.used, 0)

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
        actions={
          <>
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              {isRefreshing ? <WorkingIcon className="h-4 w-4" /> : <RefreshIcon className="h-4 w-4" />}
              {isRefreshing ? 'Refreshing' : 'Refresh'}
            </Button>
            <Link to="/storage/create" className={buttonVariants({ size: 'sm' })}>
              <AddIcon className="h-4 w-4" />
              Create pool
            </Link>
          </>
        }
      />

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as any)}>
        <TabsList>
          <TabsTrigger value="pools">Storage pools</TabsTrigger>
          <TabsTrigger value="devices">Devices</TabsTrigger>
        </TabsList>

        <TabsContent value="pools" className="space-y-6">
          {/* Pool Summary Cards */}
          {pools && pools.length > 0 && (
            // One ruled row of three equal cells: a label and its number.
            <div className="grid gap-px border border-hairline bg-hairline md:grid-cols-3 [&>*]:bg-page [&>*]:p-6">
              <div>
                <p className="font-mono text-[0.8125rem] text-ink-2">Total capacity</p>
                <p className="pixel-display pixel-24 mt-2">{formatBytes(totalSize)}</p>
              </div>
              <div>
                <p className="font-mono text-[0.8125rem] text-ink-2">Used space</p>
                <p className="pixel-display pixel-24 mt-2">{formatBytes(totalUsed)}</p>
                <Meter
                  className="mt-3"
                  value={totalUsed}
                  max={totalSize}
                  blocks={16}
                  label="Space used in all pools"
                  tone={totalSize > 0 && totalUsed / totalSize >= 0.9 ? 'failed' : totalSize > 0 && totalUsed / totalSize >= 0.8 ? 'warning' : 'ink'}
                />
              </div>
              <div>
                <p className="font-mono text-[0.8125rem] text-ink-2">Pools mounted</p>
                <p className="pixel-display pixel-24 mt-2">
                  {pools.filter((p) => p.mount).length} of {pools.length}
                </p>
              </div>
            </div>
          )}

          {/* Pools Table */}
          <Card
            title="Storage pools"
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
                icon={PoolIcon}
                title="No storage pools"
                description="A pool joins disks into one place for your shares and apps."
                action={
                  <Link to="/storage/create" className={buttonVariants()}>
                    Create pool
                  </Link>
                }
              />
            )}
          </Card>

          {/* Maintenance Operations */}
          {pools && pools.length > 0 && (
            <div className="grid gap-6 md:grid-cols-2">
              {/* Scrub Status */}
              <Card
                title="Data scrub"
                description="Verify data integrity"
                actions={
                  pools[0] && (
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => handleStartScrub(pools[0].id)}
                      disabled={scrubStatus?.some(s => s.status === 'running')}
                    >
                      Start scrub
                    </Button>
                  )
                }
              >
                {scrubStatus && scrubStatus.length > 0 ? (
                  <div className="space-y-3">
                    {scrubStatus.map((scrub) => (
                      <div key={scrub.poolId} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-sm">{scrub.poolId}</span>
                          <StatusMark state={scrub.status === 'running' ? 'running' : scrub.status === 'finished' && !scrub.errors ? 'ok' : scrub.errors ? 'failed' : 'off'}>
                            {scrub.status === 'running' ? 'Running' : scrub.status === 'finished' ? (scrub.errors ? 'Errors found' : 'Finished') : 'Idle'}
                          </StatusMark>
                        </div>
                        {scrub.progress !== undefined && scrub.status === 'running' && (
                          <Meter value={scrub.progress} kind="progress" label="Scrub progress" />
                        )}
                        {scrub.status === 'finished' && scrub.finishedAt && (
                          <p className="text-xs text-muted-foreground">
                            Last finished {formatDateTime(scrub.finishedAt)}
                            {scrub.errors ? `, ${scrub.errors} errors` : ', no errors'}
                          </p>
                        )}
                        {scrub.error && <p className="text-xs text-status-failed-text">{scrub.error}</p>}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No scrub has run yet.
                  </p>
                )}
              </Card>

              {/* Balance Status */}
              <Card
                title="Data balance"
                description="Redistribute data across devices"
                actions={
                  pools[0] && (
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => handleStartBalance(pools[0].id)}
                      disabled={balanceStatus?.some(b => b.status === 'running')}
                    >
                      Start balance
                    </Button>
                  )
                }
              >
                {balanceStatus && balanceStatus.length > 0 ? (
                  <div className="space-y-3">
                    {balanceStatus.map((balance) => (
                      <div key={balance.poolId} className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="font-mono text-sm">{balance.poolId}</span>
                          <StatusMark state={balance.status === 'running' ? 'running' : 'off'}>
                            {balance.status === 'running' ? 'Running' : 'Idle'}
                          </StatusMark>
                        </div>
                        {balance.progress !== undefined && balance.status === 'running' && (
                          <Meter value={balance.progress} kind="progress" label="Balance progress" />
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No balance is running.
                  </p>
                )}
              </Card>
            </div>
          )}
        </TabsContent>

        <TabsContent value="devices" className="space-y-6">
          {/* Devices Table */}
          <Card
            title="Storage devices"
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
                icon={PoolIcon}
                title="No devices found"
                description="No storage devices were detected in the system"
              />
            )}
          </Card>

          {/* SMART Details */}
          {selectedDevice && (
            <Card
              title="SMART details"
              description={selectedDevice}
            >
              {smartError && <p className="text-sm text-status-failed-text">{getErrorMessage(smartError)}</p>}
              {smartData ? (
                <div className="space-y-4">
                  <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
                    <dt className="text-ink-2">Overall health</dt>
                    <dd>
                      <StatusMark state={smartData.health === 'good' ? 'ok' : smartData.health === 'warning' ? 'warning' : smartData.health === 'critical' ? 'failed' : 'off'}>
                        {smartData.health === 'unknown' ? 'No SMART data' : smartData.health}
                      </StatusMark>
                    </dd>
                    {smartData.temperature_celsius !== undefined && (
                      <>
                        <dt className="text-ink-2">Temperature</dt>
                        <dd className="font-mono tabular-nums">{smartData.temperature_celsius} °C</dd>
                      </>
                    )}
                    {smartData.power_on_hours !== undefined && (
                      <>
                        <dt className="text-ink-2">Powered on</dt>
                        <dd className="font-mono tabular-nums">{smartData.power_on_hours} hours</dd>
                      </>
                    )}
                  </dl>
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