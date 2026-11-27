import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { HardDrive, KeyRound, Plus, Send, Server, Trash2, Wifi, X } from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toast } from '@/components/ui/toast'
import { getErrorMessage } from '@/lib/nos-client'
import { backupApi } from '@/api/backup'
import type { BackupJob, CreateDestinationRequest, Destination, JobState } from '@/api/backup.types'

// The backend's rules (pkg/backup/replication.go).
const HOST_PATTERN = /^[A-Za-z0-9][A-Za-z0-9.:-]{0,252}$/
const USER_PATTERN = /^[a-z_][a-z0-9_-]{0,31}$/
const PATH_PATTERN = /^\/[A-Za-z0-9._/-]{0,254}$/

const keys = {
  destinations: ['backup', 'destinations'] as const,
  jobs: ['backup', 'jobs'] as const,
  snapshots: ['backup', 'snapshots'] as const,
}

function target(d: Destination): string {
  if (d.type === 'ssh')
    return `${d.user}@${d.host}${d.port && d.port !== 22 ? `:${d.port}` : ''}:${d.path}`
  return d.path ?? ''
}

const stateBadge: Record<
  JobState,
  'default' | 'secondary' | 'success' | 'destructive' | 'outline'
> = {
  pending: 'secondary',
  running: 'default',
  succeeded: 'success',
  failed: 'destructive',
  canceled: 'outline',
}

function AddDestinationDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const qc = useQueryClient()
  const [type, setType] = useState<'ssh' | 'local'>('ssh')
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [port, setPort] = useState('22')
  const [user, setUser] = useState('')
  const [path, setPath] = useState('')
  const [key, setKey] = useState('')
  const create = useMutation({
    mutationFn: async (req: CreateDestinationRequest) => {
      const dest = await backupApi.destinations.create(req)
      if (req.type === 'ssh' && key.trim())
        await backupApi.destinations.storeSSHKey(dest.id, { key: key.trim() + '\n' })
      return dest
    },
    onSuccess: (d) => {
      qc.invalidateQueries({ queryKey: keys.destinations })
      toast.success(`Destination ${d.name} added`)
      close()
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  })

  const close = () => {
    setName('')
    setHost('')
    setPort('22')
    setUser('')
    setPath('')
    setKey('')
    onOpenChange(false)
  }
  const portNum = Number(port)
  const valid =
    name.trim() !== '' &&
    PATH_PATTERN.test(path) &&
    (type === 'local' ||
      (HOST_PATTERN.test(host) &&
        USER_PATTERN.test(user) &&
        portNum >= 1 &&
        portNum <= 65535 &&
        key.trim() !== ''))
  const submit = () =>
    create.mutate(
      type === 'ssh'
        ? { name: name.trim(), type, enabled: true, host, port: portNum, user, path }
        : { name: name.trim(), type, enabled: true, path },
    )

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Add a backup destination</DialogTitle>
          <DialogDescription>
            Snapshots are copied there with Btrfs send, so the target must be on Btrfs.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="Destination type">
            {(['ssh', 'local'] as const).map((t) => (
              <button
                key={t}
                type="button"
                role="radio"
                aria-checked={type === t}
                onClick={() => setType(t)}
                className={`flex items-center gap-2 rounded-lg border p-3 text-left text-sm ${type === t ? 'border-primary bg-primary/5' : 'border-input'}`}
              >
                {t === 'ssh' ? (
                  <Server className="h-4 w-4" aria-hidden />
                ) : (
                  <HardDrive className="h-4 w-4" aria-hidden />
                )}
                <span>
                  <span className="block font-medium">
                    {t === 'ssh' ? 'Another machine (SSH)' : 'A disk on this system'}
                  </span>
                </span>
              </button>
            ))}
          </div>
          <div className="space-y-2">
            <Label htmlFor="dest-name">Name</Label>
            <Input
              id="dest-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Office NAS"
            />
          </div>
          {type === 'ssh' && (
            <div className="grid grid-cols-[1fr_6rem] gap-2">
              <div className="space-y-2">
                <Label htmlFor="dest-host">Host</Label>
                <Input
                  id="dest-host"
                  value={host}
                  onChange={(e) => setHost(e.target.value.trim())}
                  placeholder="backup.example.com"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="dest-port">Port</Label>
                <Input
                  id="dest-port"
                  inputMode="numeric"
                  value={port}
                  onChange={(e) => setPort(e.target.value.replace(/\D/g, ''))}
                />
              </div>
            </div>
          )}
          {type === 'ssh' && (
            <div className="space-y-2">
              <Label htmlFor="dest-user">User on that machine</Label>
              <Input id="dest-user" value={user} onChange={(e) => setUser(e.target.value.trim())} />
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="dest-path">
              {type === 'ssh' ? 'Folder on that machine' : 'Folder'}
            </Label>
            <Input
              id="dest-path"
              value={path}
              onChange={(e) => setPath(e.target.value.trim())}
              placeholder={type === 'ssh' ? '/backups/nos' : '/mnt/backup-disk/nos'}
            />
            {path && !PATH_PATTERN.test(path) && (
              <p className="text-sm text-destructive">
                An absolute path of letters, digits, '.', '_', '-' and '/'
              </p>
            )}
          </div>
          {type === 'ssh' && (
            <div className="space-y-2">
              <Label htmlFor="dest-key">Private key</Label>
              <textarea
                id="dest-key"
                className="h-28 w-full rounded-md border border-input bg-background p-2 font-mono text-xs"
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                spellCheck={false}
              />
              <p className="text-sm text-muted-foreground">
                The key nOS signs in with; its public half must be in that user's authorized_keys.
                The host's key is remembered on the first connection.
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={close}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={!valid || create.isPending}>
            {create.isPending ? 'Adding…' : 'Add destination'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function SSHKeyDialog({ dest, onClose }: { dest: Destination | null; onClose: () => void }) {
  const [key, setKey] = useState('')
  const save = useMutation({
    mutationFn: () => backupApi.destinations.storeSSHKey(dest!.id, { key: key.trim() + '\n' }),
    onSuccess: () => {
      toast.success('Key saved')
      setKey('')
      onClose()
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  })
  return (
    <Dialog open={dest !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Replace the key for {dest?.name}</DialogTitle>
          <DialogDescription>
            The private key nOS signs in to {dest ? target(dest) : ''} with.
          </DialogDescription>
        </DialogHeader>
        <textarea
          aria-label="Private key"
          className="h-32 w-full rounded-md border border-input bg-background p-2 font-mono text-xs"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          spellCheck={false}
        />
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => save.mutate()} disabled={!key.trim() || save.isPending}>
            Save key
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function CopyNowDialog({ dest, onClose }: { dest: Destination | null; onClose: () => void }) {
  const qc = useQueryClient()
  const snapshots = useQuery({
    queryKey: keys.snapshots,
    queryFn: () => backupApi.snapshots.list(),
    enabled: dest !== null,
  })
  const [snapshotId, setSnapshotId] = useState('')
  const list = (snapshots.data?.snapshots ?? [])
    .slice()
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
  const replicate = useMutation({
    mutationFn: () => backupApi.replicate({ destination_id: dest!.id, snapshot_id: snapshotId }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: keys.jobs })
      toast.success('Copy started')
      setSnapshotId('')
      onClose()
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  })
  return (
    <Dialog open={dest !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Copy a snapshot to {dest?.name}</DialogTitle>
          <DialogDescription>
            The copy runs in the background; its progress is listed below.
          </DialogDescription>
        </DialogHeader>
        {snapshots.isLoading && <p className="text-sm text-muted-foreground">Loading snapshots…</p>}
        {snapshots.error && (
          <p className="text-sm text-destructive">{getErrorMessage(snapshots.error)}</p>
        )}
        {!snapshots.isLoading && list.length === 0 && (
          <p className="text-sm text-muted-foreground">
            There are no snapshots yet. Take one under Backup › Snapshots.
          </p>
        )}
        {list.length > 0 && (
          <select
            aria-label="Snapshot"
            className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
            value={snapshotId}
            onChange={(e) => setSnapshotId(e.target.value)}
          >
            <option value="">Choose a snapshot</option>
            {list.map((s) => (
              <option key={s.id} value={s.id}>
                {s.subvolume} · {new Date(s.created_at).toLocaleString()}
              </option>
            ))}
          </select>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => replicate.mutate()} disabled={!snapshotId || replicate.isPending}>
            <Send className="mr-2 h-4 w-4" aria-hidden />
            Copy
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DestinationsCard() {
  const qc = useQueryClient()
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: keys.destinations,
    queryFn: backupApi.destinations.list,
  })
  const [adding, setAdding] = useState(false)
  const [keyFor, setKeyFor] = useState<Destination | null>(null)
  const [copyTo, setCopyTo] = useState<Destination | null>(null)
  const refresh = () => qc.invalidateQueries({ queryKey: keys.destinations })
  const test = useMutation({
    mutationFn: (d: Destination) => backupApi.destinations.test(d.id),
    onSuccess: (r, d) => {
      if (r.success) toast.success(`${d.name} is reachable`)
      else toast.error(`${d.name}: ${r.error ?? 'not reachable'}`)
      refresh()
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  })
  const remove = useMutation({
    mutationFn: (d: Destination) => backupApi.destinations.delete(d.id),
    onSuccess: (_, d) => {
      toast.success(`${d.name} removed; copies already there stay`)
      refresh()
    },
    onError: (e) => toast.error(getErrorMessage(e)),
  })
  const list = data?.destinations ?? []

  return (
    <Card
      title="Destinations"
      description="Where snapshots are copied to"
      isLoading={isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => refetch()}
      actions={
        <Button size="sm" onClick={() => setAdding(true)}>
          <Plus className="mr-2 h-4 w-4" aria-hidden />
          Add destination
        </Button>
      }
    >
      {list.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No destinations yet. Add another machine over SSH, or a second disk, to keep copies of
          your snapshots off this one.
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Where</TableHead>
              <TableHead>Last check</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.map((d) => (
              <TableRow key={d.id}>
                <TableCell className="font-medium">{d.name}</TableCell>
                <TableCell className="font-mono text-xs">{target(d)}</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {d.last_test ? new Date(d.last_test).toLocaleString() : 'Never'}
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-1">
                    <Button variant="outline" size="sm" onClick={() => setCopyTo(d)}>
                      <Send className="mr-1 h-4 w-4" aria-hidden />
                      Copy now
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => test.mutate(d)}
                      disabled={test.isPending}
                      title="Check the connection"
                    >
                      <Wifi className="h-4 w-4" aria-hidden />
                      <span className="sr-only">Check the connection to {d.name}</span>
                    </Button>
                    {d.type === 'ssh' && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setKeyFor(d)}
                        title="Replace the SSH key"
                      >
                        <KeyRound className="h-4 w-4" aria-hidden />
                        <span className="sr-only">Replace the SSH key of {d.name}</span>
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => remove.mutate(d)}
                      disabled={remove.isPending}
                      title="Remove"
                    >
                      <Trash2 className="h-4 w-4 text-destructive" aria-hidden />
                      <span className="sr-only">Remove {d.name}</span>
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <AddDestinationDialog open={adding} onOpenChange={setAdding} />
      <SSHKeyDialog dest={keyFor} onClose={() => setKeyFor(null)} />
      <CopyNowDialog dest={copyTo} onClose={() => setCopyTo(null)} />
    </Card>
  )
}

function CopiesCard() {
  const qc = useQueryClient()
  const destinations = useQuery({
    queryKey: keys.destinations,
    queryFn: backupApi.destinations.list,
  })
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: keys.jobs,
    queryFn: () => backupApi.jobs.list(50),
    // Poll while a copy runs.
    refetchInterval: (q) =>
      (q.state.data?.jobs ?? []).some(
        (j: BackupJob) => j.state === 'running' || j.state === 'pending',
      )
        ? 3000
        : false,
  })
  const cancel = useMutation({
    mutationFn: (id: string) => backupApi.jobs.cancel(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.jobs }),
    onError: (e) => toast.error(getErrorMessage(e)),
  })
  const names = new Map((destinations.data?.destinations ?? []).map((d) => [d.id, d.name]))
  const jobs = (data?.jobs ?? []).filter((j) => j.type === 'replicate')

  return (
    <Card
      title="Recent copies"
      isLoading={isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => refetch()}
    >
      {jobs.length === 0 ? (
        <p className="text-sm text-muted-foreground">No copies yet.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Started</TableHead>
              <TableHead>To</TableHead>
              <TableHead>State</TableHead>
              <TableHead className="text-right" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {jobs.map((j) => (
              <TableRow key={j.id}>
                <TableCell className="text-sm">{new Date(j.started_at).toLocaleString()}</TableCell>
                <TableCell className="text-sm">
                  {names.get(j.destination_id ?? '') ?? j.destination_id}
                </TableCell>
                <TableCell>
                  <Badge variant={stateBadge[j.state]}>
                    {j.state}
                    {j.state === 'running' ? ` ${Math.round(j.progress)}%` : ''}
                  </Badge>
                  {j.error && <p className="mt-1 text-xs text-destructive">{j.error}</p>}
                </TableCell>
                <TableCell className="text-right">
                  {(j.state === 'running' || j.state === 'pending') && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => cancel.mutate(j.id)}
                      disabled={cancel.isPending}
                    >
                      <X className="mr-1 h-4 w-4" aria-hidden />
                      Stop
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Card>
  )
}

/**
 * Off-site copies: snapshots sent to another machine or disk. (This page used
 * to call a remote-backup API that never existed, /remote/destinations and
 * /remote/jobs; it now uses the backup service's destinations and
 * replication.)
 */
export function Remote() {
  return (
    <div className="space-y-6">
      <PageHeader
        title="Off-site copies"
        description="Copies of your snapshots on another machine or disk"
      />
      <DestinationsCard />
      <CopiesCard />
    </div>
  )
}
