import { useState } from 'react'
import { motion } from 'framer-motion'
import { formatDistanceToNow } from 'date-fns'
import { 
  Cloud,
  Plus,
  Edit,
  Trash2,
  Play,
  Pause,
  Calendar,
  Clock,
  CheckCircle,
  XCircle,
  Server,
  Shield,
  Key,
  FileText,
  Globe,
  Lock,
  RefreshCw,
  AlertCircle,
  HardDrive,
  Database,
} from 'lucide-react'
import { ColumnDef } from '@tanstack/react-table'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { EmptyState } from '@/components/ui/empty-state'
import { StatusPill, Metric } from '@/components/ui/status'
import { Button } from '@/components/ui/button'
import { DataTable } from '@/components/ui/data-table'
import { SlideOver } from '@/components/ui/slide-over'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'
import { toast } from '@/components/ui/toast'
import {
  useRemoteDestinations,
  useCreateDestination,
  useDeleteDestination,
  useBackupJobs,
  useStartBackupJob,
  useStopBackupJob,
  useBackupStats,
} from '@/hooks/use-api'

// Destination columns
const destinationColumns: ColumnDef<any>[] = [
  {
    accessorKey: 'name',
    header: 'Destination',
    cell: ({ row }) => (
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-lg bg-muted">
          {row.original.type === 's3' ? <Cloud className="h-4 w-4 text-muted-foreground" /> :
           row.original.type === 'ssh' ? <Server className="h-4 w-4 text-muted-foreground" /> :
           row.original.type === 'webdav' ? <Globe className="h-4 w-4 text-muted-foreground" /> :
           <Database className="h-4 w-4 text-muted-foreground" />}
        </div>
        <div>
          <div className="font-medium">{row.original.name}</div>
          <div className="text-xs text-muted-foreground">{row.original.endpoint}</div>
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'type',
    header: 'Type',
    cell: ({ row }) => (
      <StatusPill variant="info">
        {row.original.type.toUpperCase()}
      </StatusPill>
    ),
  },
  {
    accessorKey: 'encrypted',
    header: 'Encryption',
    cell: ({ row }) => (
      <div className="flex items-center gap-2">
        {row.original.encrypted ? (
          <>
            <Lock className="h-4 w-4 text-green-500" />
            <span className="text-sm">Enabled</span>
          </>
        ) : (
          <>
            <Lock className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">Disabled</span>
          </>
        )}
      </div>
    ),
  },
  {
    accessorKey: 'lastSync',
    header: 'Last Sync',
    cell: ({ row }) => {
      const lastSync = row.original.lastSync
      return lastSync ? (
        <div className="text-sm">
          {formatDistanceToNow(new Date(lastSync), { addSuffix: true })}
        </div>
      ) : (
        <span className="text-sm text-muted-foreground">Never</span>
      )
    },
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => {
      const status = row.original.status
      return (
        <StatusPill variant={
          status === 'active' ? 'success' :
          status === 'error' ? 'error' : 
          status === 'testing' ? 'warning' : 'muted'
        }>
          {status}
        </StatusPill>
      )
    },
  },
]

// Job columns
const jobColumns: ColumnDef<any>[] = [
  {
    accessorKey: 'name',
    header: 'Job',
    cell: ({ row }) => (
      <div>
        <div className="font-medium">{row.original.name}</div>
        <div className="text-xs text-muted-foreground">
          {row.original.source} → {row.original.destination}
        </div>
      </div>
    ),
  },
  {
    accessorKey: 'schedule',
    header: 'Schedule',
    cell: ({ row }) => (
      <div className="flex items-center gap-2">
        <Calendar className="h-4 w-4 text-muted-foreground" />
        <span className="text-sm font-mono">{row.original.schedule || 'Manual'}</span>
      </div>
    ),
  },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => {
      const { status, progress } = row.original
      if (status === 'running' && progress !== undefined) {
        return (
          <div className="space-y-1">
            <StatusPill variant="info">Running</StatusPill>
            <div className="w-24 h-1.5 bg-muted rounded-full overflow-hidden">
              <div 
                className="h-full bg-primary transition-all"
                style={{ width: `${progress}%` }}
              />
            </div>
            <span className="text-xs text-muted-foreground">{progress}%</span>
          </div>
        )
      }
      return (
        <StatusPill variant={
          status === 'idle' ? 'muted' : 
          status === 'running' ? 'info' :
          status === 'completed' ? 'success' :
          status === 'failed' ? 'error' : 'warning'
        }>
          {status}
        </StatusPill>
      )
    },
  },
  {
    accessorKey: 'lastResult',
    header: 'Last Result',
    cell: ({ row }) => {
      const result = row.original.lastResult
      const lastRun = row.original.lastRun
      return result ? (
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            {result === 'success' ? (
              <CheckCircle className="h-4 w-4 text-green-500" />
            ) : result === 'failed' ? (
              <XCircle className="h-4 w-4 text-red-500" />
            ) : (
              <AlertCircle className="h-4 w-4 text-yellow-500" />
            )}
            <span className="text-sm capitalize">{result}</span>
          </div>
          {lastRun && (
            <div className="text-xs text-muted-foreground">
              {formatDistanceToNow(new Date(lastRun), { addSuffix: true })}
            </div>
          )}
        </div>
      ) : (
        <span className="text-sm text-muted-foreground">-</span>
      )
    },
  },
  {
    accessorKey: 'nextRun',
    header: 'Next Run',
    cell: ({ row }) => {
      const date = row.original.nextRun ? new Date(row.original.nextRun) : null
      return date ? (
        <div className="flex items-center gap-2 text-sm">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <span>{formatDistanceToNow(date, { addSuffix: true })}</span>
        </div>
      ) : (
        <span className="text-sm text-muted-foreground">-</span>
      )
    },
  },
]

// Destination form component
function DestinationForm({ 
  onSubmit, 
  onCancel,
  destination = null 
}: { 
  onSubmit: (data: any) => void
  onCancel: () => void
  destination?: any
}) {
  const [formData, setFormData] = useState({
    name: destination?.name || '',
    type: destination?.type || 's3',
    endpoint: destination?.endpoint || '',
    bucket: destination?.bucket || '',
    path: destination?.path || '',
    accessKey: destination?.accessKey || '',
    secretKey: '',
    encrypted: destination?.encrypted ?? true,
    compression: destination?.compression ?? true,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit(formData)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium mb-2">Name</label>
        <input
          type="text"
          value={formData.name}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          placeholder="e.g., AWS Backup"
          required
        />
      </div>

      <div>
        <label className="block text-sm font-medium mb-2">Type</label>
        <div className="grid grid-cols-2 gap-2">
          {['s3', 'ssh', 'webdav', 'local'].map(type => (
            <button
              key={type}
              type="button"
              onClick={() => setFormData({ ...formData, type })}
              className={cn(
                "p-3 rounded-lg border transition-all flex items-center gap-2",
                formData.type === type
                  ? "border-primary bg-primary/10"
                  : "border-border hover:bg-muted/50"
              )}
            >
              {type === 's3' && <Cloud className="h-4 w-4" />}
              {type === 'ssh' && <Server className="h-4 w-4" />}
              {type === 'webdav' && <Globe className="h-4 w-4" />}
              {type === 'local' && <HardDrive className="h-4 w-4" />}
              <span className="uppercase text-sm font-medium">{type}</span>
            </button>
          ))}
        </div>
      </div>

      {formData.type !== 'local' && (
        <div>
          <label className="block text-sm font-medium mb-2">Endpoint</label>
          <input
            type="text"
            value={formData.endpoint}
            onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            placeholder={
              formData.type === 's3' ? 's3.amazonaws.com' :
              formData.type === 'ssh' ? '192.168.1.100:22' :
              'https://webdav.example.com'
            }
            required
          />
        </div>
      )}

      {formData.type === 's3' && (
        <>
          <div>
            <label className="block text-sm font-medium mb-2">Bucket</label>
            <input
              type="text"
              value={formData.bucket}
              onChange={(e) => setFormData({ ...formData, bucket: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              placeholder="my-backup-bucket"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">Access Key</label>
            <input
              type="text"
              value={formData.accessKey}
              onChange={(e) => setFormData({ ...formData, accessKey: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              placeholder="AKIAIOSFODNN7EXAMPLE"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium mb-2">Secret Key</label>
            <input
              type="password"
              value={formData.secretKey}
              onChange={(e) => setFormData({ ...formData, secretKey: e.target.value })}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
              placeholder="••••••••••••••••"
              required={!destination}
            />
            {destination && (
              <p className="text-xs text-muted-foreground mt-1">
                Leave blank to keep existing key
              </p>
            )}
          </div>
        </>
      )}

      {(formData.type === 'ssh' || formData.type === 'webdav' || formData.type === 'local') && (
        <div>
          <label className="block text-sm font-medium mb-2">Path</label>
          <input
            type="text"
            value={formData.path}
            onChange={(e) => setFormData({ ...formData, path: e.target.value })}
            className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
            placeholder="/backup/nos"
            required
          />
        </div>
      )}

      <div className="space-y-3">
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={formData.encrypted}
            onChange={(e) => setFormData({ ...formData, encrypted: e.target.checked })}
            className="rounded"
          />
          <span className="text-sm">Enable encryption</span>
        </label>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={formData.compression}
            onChange={(e) => setFormData({ ...formData, compression: e.target.checked })}
            className="rounded"
          />
          <span className="text-sm">Enable compression</span>
        </label>
      </div>

      <div className="flex justify-end gap-2 pt-4 border-t">
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit">
          {destination ? 'Update' : 'Create'} Destination
        </Button>
      </div>
    </form>
  )
}

// Helper function
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export function Remote() {
  const [isDestinationOpen, setIsDestinationOpen] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [selectedDestination, setSelectedDestination] = useState<any>(null)
  const [selectedJob, setSelectedJob] = useState<any>(null)

  // Fetch data using real hooks
  const { data: destinations = [], isLoading: destinationsLoading, refetch: refetchDestinations } = useRemoteDestinations()
  const { data: jobs = [], isLoading: jobsLoading, refetch: refetchJobs } = useBackupJobs()
  const { data: stats } = useBackupStats()
  
  // Mutations
  const createDestination = useCreateDestination()
  const deleteDestination = useDeleteDestination()
  const startJob = useStartBackupJob()
  const stopJob = useStopBackupJob()

  const handleCreateDestination = async (data: any) => {
    try {
      await createDestination.mutateAsync(data)
      toast.success('Destination created successfully')
      setIsDestinationOpen(false)
      setSelectedDestination(null)
    } catch (error) {
      toast.error('Failed to create destination')
      console.error(error)
    }
  }

  const handleDeleteDestination = async (id: string) => {
    if (!confirm('Are you sure you want to delete this destination?')) return
    
    try {
      await deleteDestination.mutateAsync(id)
      toast.success('Destination deleted successfully')
    } catch (error) {
      toast.error('Failed to delete destination')
      console.error(error)
    }
  }

  const handleJobAction = async (jobId: string, action: 'start' | 'stop') => {
    try {
      if (action === 'start') {
        await startJob.mutateAsync(jobId)
        toast.success('Job started successfully')
      } else {
        await stopJob.mutateAsync(jobId)
        toast.success('Job stopped successfully')
      }
    } catch (error) {
      toast.error(`Failed to ${action} job`)
      console.error(error)
    }
  }

  const handleRefresh = async () => {
    setIsRefreshing(true)
    await Promise.all([refetchDestinations(), refetchJobs()])
    setTimeout(() => setIsRefreshing(false), 500)
  }

  // Calculate stats from real data
  const totalBackupSize = (stats as any)?.totalBackupSize || 0
  const successRate = (stats as any)?.successRate || 0
  const activeJobs = (jobs as any[]).filter((j: any) => j.status === 'running').length
  const activeDestinations = (destinations as any[]).filter((d: any) => d.status === 'active').length

  // Enhanced columns with actions
  const enhancedDestinationColumns = [
    ...destinationColumns,
    {
      id: 'actions',
      header: 'Actions',
      cell: ({ row }: any) => (
        <div className="flex items-center gap-1">
          <Button 
            variant="ghost" 
            size="sm"
            onClick={() => {
              setSelectedDestination(row.original)
              setIsDestinationOpen(true)
            }}
          >
            <Edit className="h-4 w-4" />
          </Button>
          <Button 
            variant="ghost" 
            size="sm" 
            className="text-destructive"
            onClick={() => handleDeleteDestination(row.original.id)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ),
    },
  ]

  const enhancedJobColumns = [
    ...jobColumns,
    {
      id: 'actions',
      header: 'Actions',
      cell: ({ row }: any) => {
        const isRunning = row.original.status === 'running'
        return (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleJobAction(row.original.id, isRunning ? 'stop' : 'start')}
              disabled={startJob.isPending || stopJob.isPending}
            >
              {isRunning ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
            </Button>
            <Button 
              variant="ghost" 
              size="sm"
              onClick={() => setSelectedJob(row.original)}
            >
              <FileText className="h-4 w-4" />
            </Button>
          </div>
        )
      },
    },
  ]

  return (
    <div className="space-y-6">
      <PageHeader
        title="Remote Backup"
        description="Manage backup destinations and replication jobs"
        actions={
          <div className="flex items-center gap-2">
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
              onClick={() => {
                setSelectedDestination(null)
                setIsDestinationOpen(true)
              }}
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Destination
            </Button>
          </div>
        }
      />

      {/* Stats */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="grid gap-4 md:grid-cols-4"
      >
        <Card>
          <Metric
            label="Total Destinations"
            value={(destinations as any[]).length}
            sublabel={`${activeDestinations} active`}
          />
        </Card>
        <Card>
          <Metric
            label="Backup Jobs"
            value={(jobs as any[]).length}
            sublabel={`${activeJobs} running`}
          />
        </Card>
        <Card>
          <Metric
            label="Success Rate"
            value={`${Math.round(successRate)}%`}
            sublabel="Last 24 hours"
          />
        </Card>
        <Card>
          <Metric
            label="Total Size"
            value={formatBytes(totalBackupSize)}
            sublabel="Backed up data"
          />
        </Card>
      </motion.div>

      {/* Destinations */}
      <Card
        title="Backup Destinations"
        description="Remote storage locations for your backups"
        isLoading={destinationsLoading}
      >
        {(destinations as any[]).length > 0 ? (
          <DataTable
            columns={enhancedDestinationColumns}
            data={destinations as any[]}
            searchKey="name"
          />
        ) : (
          <EmptyState
            variant="no-data"
            icon={Cloud}
            title="No destinations configured"
            description="Add a backup destination to get started with remote backups"
            action={{
              label: "Add Destination",
              onClick: () => {
                setSelectedDestination(null)
                setIsDestinationOpen(true)
              }
            }}
          />
        )}
      </Card>

      {/* Jobs */}
      <Card
        title="Backup Jobs"
        description="Scheduled backup and replication tasks"
        isLoading={jobsLoading}
      >
        {(jobs as any[]).length > 0 ? (
          <DataTable
            columns={enhancedJobColumns}
            data={jobs as any[]}
            searchKey="name"
          />
        ) : (
          <EmptyState
            variant="no-data"
            icon={Calendar}
            title="No backup jobs"
            description="Create a backup job to start protecting your data"
            action={{
              label: "Create Job",
              onClick: () => toast.info('Job creation coming soon!')
            }}
          />
        )}
      </Card>

      {/* Encryption settings */}
      <Card
        title="Security Settings"
        description="Manage backup encryption and authentication"
      >
        <div className="space-y-4">
          <div className="flex items-start gap-3 p-4 rounded-lg bg-muted/30">
            <Shield className="h-5 w-5 text-primary mt-0.5" />
            <div className="flex-1">
              <h4 className="font-medium">End-to-End Encryption</h4>
              <p className="text-sm text-muted-foreground mt-1">
                All backups are encrypted before leaving your system using AES-256 encryption.
                Your encryption keys never leave your device.
              </p>
            </div>
            <Button variant="outline" size="sm">
              <Key className="h-4 w-4 mr-2" />
              Manage Keys
            </Button>
          </div>
          
          <Alert>
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              <strong>Important:</strong> Store your encryption keys safely. Lost keys cannot be recovered,
              and encrypted backups cannot be restored without them.
            </AlertDescription>
          </Alert>
        </div>
      </Card>

      {/* Add/Edit Destination SlideOver */}
      <SlideOver
        isOpen={isDestinationOpen}
        onClose={() => {
          setIsDestinationOpen(false)
          setSelectedDestination(null)
        }}
        title={selectedDestination ? "Edit Backup Destination" : "Add Backup Destination"}
        description={selectedDestination ? "Update remote storage configuration" : "Configure a new remote storage location"}
        size="md"
      >
        <DestinationForm
          destination={selectedDestination}
          onSubmit={handleCreateDestination}
          onCancel={() => {
            setIsDestinationOpen(false)
            setSelectedDestination(null)
          }}
        />
      </SlideOver>

      {/* Job Details SlideOver */}
      {selectedJob && (
        <SlideOver
          isOpen={!!selectedJob}
          onClose={() => setSelectedJob(null)}
          title="Job Details"
          description={selectedJob.name}
          size="md"
        >
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-muted-foreground">Status</p>
                <p className="font-medium">{selectedJob.status}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Schedule</p>
                <p className="font-medium">{selectedJob.schedule || 'Manual'}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Source</p>
                <p className="font-medium">{selectedJob.source}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Destination</p>
                <p className="font-medium">{selectedJob.destination}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Last Run</p>
                <p className="font-medium">
                  {selectedJob.lastRun 
                    ? formatDistanceToNow(new Date(selectedJob.lastRun), { addSuffix: true })
                    : 'Never'}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground">Next Run</p>
                <p className="font-medium">
                  {selectedJob.nextRun 
                    ? formatDistanceToNow(new Date(selectedJob.nextRun), { addSuffix: true })
                    : 'Not scheduled'}
                </p>
              </div>
            </div>
            
            {selectedJob.logs && (
              <div>
                <h4 className="font-medium mb-2">Recent Logs</h4>
                <div className="bg-muted rounded-lg p-3 max-h-64 overflow-y-auto">
                  <pre className="text-xs font-mono">{selectedJob.logs}</pre>
                </div>
              </div>
            )}
            
            <div className="flex justify-end gap-2 pt-4 border-t">
              <Button variant="outline" onClick={() => setSelectedJob(null)}>
                Close
              </Button>
            </div>
          </div>
        </SlideOver>
      )}
    </div>
  )
}