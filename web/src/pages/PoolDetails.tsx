import { useEffect, useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { PoolSnapshots } from './PoolSnapshots'
import http, { api } from '@/lib/nos-client'
import type { TxLogResponse } from '@/types/api'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Meter, Working } from '@/components/ui/pixel'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FailedIcon, WarningIcon } from '@/components/icons'

export function PoolDetails() {
  const { id } = useParams()
  const [pools, setPools] = useState<any[]>([])
  const pool = useMemo(() => pools.find((p) => p.id === id || p.mount === id || p.uuid === id), [pools, id])
  const [tab, setTab] = useState<'overview'|'snapshots'|'devices'>('overview')

  useEffect(() => {
    http.pools.list().then((data: any) => setPools(data))
  }, [])

  if (!id) return <div className="p-4">Missing id</div>

  return (
    <div className="space-y-6">
      <div>
        <nav aria-label="Breadcrumb" className="mb-2 text-sm text-ink-2">
          <Link to="/storage" className="underline decoration-ink-2 underline-offset-4 hover:text-ink hover:decoration-ink">Storage</Link>
          <span className="mx-2">/</span>
          <span className="font-mono text-ink">{pool?.label || id}</span>
        </nav>
        <h1 className="screen-title">Pool {pool?.label || id}</h1>
      </div>
      <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="snapshots">Snapshots</TabsTrigger>
          <TabsTrigger value="devices">Devices</TabsTrigger>
        </TabsList>
      </Tabs>

      {tab==='overview' && (
        pool ? (
          <div className="space-y-6">
            <div className="grid gap-px border border-hairline bg-hairline md:grid-cols-3 [&>*]:bg-page [&>*]:p-4">
              <Field label="Label" value={pool.label || '-'} />
              <Field label="UUID" value={pool.uuid || '-'} mono />
              <Field label="Profile" value={(pool.raid || '-').toUpperCase()} mono />
              <Field label="Mount point" value={pool.mount || pool.id} mono />
              <Field label="Size" value={formatBytes(pool.size)} mono />
              <Field label="Used / free" value={`${formatBytes(pool.used)} / ${formatBytes(pool.free)}`} mono />
            </div>
            <Meter
              value={pool.used || 0}
              max={pool.size || 0}
              label={`${pool.label || id} used`}
              tone={pool.size && pool.used / pool.size >= 0.9 ? 'failed' : pool.size && pool.used / pool.size >= 0.8 ? 'warning' : 'ink'}
            />
          </div>
        ) : (<Working>Loading the pool</Working>)
      )}

      {tab==='overview' && pool && (
        <MountOptionsCard pool={pool} id={id!} onSaved={() => { /* no refetch needed here */ }} />
      )}

      {tab==='snapshots' && (
        <section className="space-y-3">
          <h2 className="text-xl font-[560]">Snapshots</h2>
          <PoolSnapshots id={id} />
        </section>
      )}

      {tab==='devices' && (
        <DevicesWizards id={id!} />
      )}
    </div>
  )
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="font-mono text-[0.8125rem] text-ink-2">{label}</div>
      <div className={mono ? 'mt-1 break-all font-mono text-sm' : 'mt-1 text-sm'}>{value}</div>
    </div>
  )
}

function formatBytes(n: number): string {
  if (!n || Number.isNaN(n as any)) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) { v/=1024; i++ }
  return `${v.toFixed(1)} ${units[i]}`
}

function computeDefaultFromPoolDevices(pool: any): string {
  const devices: any[] = pool?.devices || []
  const allSSD = devices.length > 0 ? devices.every(d => d?.rota === false || d?.rota === 0) : false
  return allSSD ? 'compress=zstd:3,ssd,discard=async,noatime' : 'compress=zstd:3,noatime'
}

function validateTokens(s: string): string | null {
  if (!s || !s.trim()) return 'Required'
  const toks = s.split(',').map(t => t.trim()).filter(Boolean)
  const allowed = new Set(['ssd','noatime','nodiratime','autodefrag','discard','discard=async'])
  for (const t of toks) {
    const low = t.toLowerCase()
    if (low === 'nodatacow') return 'nodatacow is not allowed'
    if (low.startsWith('compress=')) {
      const rest = low.slice('compress='.length)
      if (!rest.startsWith('zstd')) return 'compress must be zstd'
      const idx = rest.indexOf(':')
      if (idx >= 0) {
        const lvl = parseInt(rest.slice(idx+1), 10)
        if (!(lvl >= 1 && lvl <= 15)) return 'compress level must be 1..15'
      }
      continue
    }
    if (!allowed.has(low)) return `Unknown option: ${t}`
  }
  return null
}

function MountOptionsCard({ pool, id, onSaved }: { pool: any; id: string; onSaved: () => void }) {
  const [current, setCurrent] = useState<string>('')
  const [open, setOpen] = useState(false)
  const [value, setValue] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let ignore = false
    Promise
      .resolve((api as any).pools?.getMountOptions ? http.pools.getMountOptions(id) : undefined)
      .then((r: any) => { if (!ignore && r && typeof r.mountOptions === 'string') setCurrent(r.mountOptions) })
      .catch(()=>{})
    return () => { ignore = true }
  }, [id])

  async function onEdit() {
    setError(null)
    try {
      const r = await http.pools.getMountOptions(id)
      setValue((r as any).mountOptions)
      setOpen(true)
    } catch (e:any) {
      try { const { toast } = await import('@/components/ui/toast'); toast.error(e?.message || 'Failed to load') } catch {}
    }
  }

  async function onSave() {
    const v = value.trim()
    const ve = validateTokens(v)
    if (ve) { setError(ve); return }
    setSaving(true)
    try {
      const r = await http.pools.updateMountOptions(id, v) as any
      setCurrent(r.mountOptions)
      setOpen(false)
      onSaved()
      try {
        const { toast } = await import('@/components/ui/toast')
        toast.success(r.rebootRequired ? 'Saved. Will take effect after reboot or remount.' : 'Saved and applied.')
      } catch {}
    } catch (e:any) {
      try { const { toast } = await import('@/components/ui/toast'); toast.error(e?.message || 'Save failed') } catch {}
    } finally { setSaving(false) }
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-[560]">Mount options</h2>
        <Button variant="outline" size="sm" onClick={onEdit}>Edit</Button>
      </div>
      <p className="break-all font-mono text-sm">{current || <Working />}</p>

      <Dialog open={open} onOpenChange={(o) => !saving && setOpen(o)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit mount options</DialogTitle>
            <DialogDescription>
              SSD: <span className="font-mono">compress=zstd:3,ssd,discard=async,noatime</span>. Disks and mixed:{' '}
              <span className="font-mono">compress=zstd:3,noatime</span>.
            </DialogDescription>
          </DialogHeader>
          <label htmlFor="mount-options" className="sr-only">Mount options</label>
          <textarea
            id="mount-options"
            className="field h-28 py-2 font-mono"
            value={value}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'mount-options-error' : undefined}
            onChange={(e) => { setValue(e.target.value); setError(null) }}
          />
          {error && (
            <p id="mount-options-error" className="flex items-center gap-2 text-sm text-status-failed-text">
              <FailedIcon className="h-4 w-4" />
              {error}
            </p>
          )}
          <DialogFooter className="gap-2">
            <Button variant="ghost" onClick={() => setValue(computeDefaultFromPoolDevices(pool))}>Restore defaults</Button>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={saving}>Cancel</Button>
            <Button onClick={onSave} disabled={saving}>{saving ? 'Saving' : 'Save'}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}

function DevicesWizards({ id }: { id: string }) {
  const [mode, setMode] = useState<'add'|'remove'|'replace'|''>('')
  const [form, setForm] = useState<any>({ add:'', remove:'', replaceOld:'', replaceNew:'', confirm:'' })
  const [plan, setPlan] = useState<any|null>(null)
  const [tx, setTx] = useState<string>('')
  const [log, setLog] = useState<string[]>([])
  const [error, setError] = useState<string>('')

  async function onPlan() {
    setError('')
    try {
      const body:any = { action: mode, devices:{}, targetProfile:{} }
      if (mode==='add') body.devices.add = form.add.split(/\s+/).filter(Boolean)
      if (mode==='remove') body.devices.remove = form.remove.split(/\s+/).filter(Boolean)
      if (mode==='replace') body.devices.replace = [{ old: form.replaceOld.trim(), new: form.replaceNew.trim() }]
      const res = await http.pools.planDevice(id, body)
      setPlan(res)
    } catch (e:any) {
      setError(e?.message || 'Plan failed')
    }
  }
  async function onApply() {
    if (!plan) return
    setError('')
    try {
      const steps = (plan.steps || []).map((s:any) => ({ id:s.id||s.ID||'', description:s.description||s.Description||'', command:s.command||s.Command||'' }))
      const confirm = mode==='add' ? 'ADD' : mode==='remove' ? 'REMOVE' : 'REPLACE'
      const res = await http.pools.applyDevice(id, { steps, confirm }) as any
      setTx(res.tx_id)
    } catch (e:any) {
      const msg = e?.message || 'Apply failed'
      setError(msg)
      // pool busy UX: surface link when possible
      if (typeof msg === 'string' && msg.includes('pool.busy')) {
        const txIdMatch = msg.match(/txId":"([^"]+)/)
        const txId = txIdMatch?.[1]
        try {
          const { toast } = await import('@/components/ui/toast')
          toast.error(txId ? `Pool is busy. View progress: /pools/tx/${txId}` : 'Pool is busy running another operation.')
        } catch {}
      }
    }
  }

  useEffect(()=>{
    let t: any
    async function poll() {
      if (!tx) return
      const r = await http.pools.txLog(tx, log.length, 1000).catch(()=>null) as TxLogResponse | null
      if (r && Array.isArray(r.lines) && typeof r.nextCursor === 'number') {
        setLog(prev => [...prev, ...r.lines])
      }
      t = setTimeout(poll, 1000)
    }
    if (tx) poll()
    return () => { if (t) clearTimeout(t) }
  }, [tx, log.length])

  return (
    <section className="space-y-3">
      <h2 className="text-xl font-[560]">Devices</h2>
      <div role="radiogroup" aria-label="Change" className="inline-flex divide-x divide-rule overflow-hidden rounded-lg border border-rule">
        {(['add', 'remove', 'replace'] as const).map((m) => (
          <button
            key={m}
            type="button"
            role="radio"
            aria-checked={mode === m}
            className={`min-h-11 px-4 text-sm capitalize ${mode === m ? 'bg-ink text-page' : 'hover:bg-ink/[.09]'}`}
            onClick={() => { setMode(m); setPlan(null); setLog([]); setTx('') }}
          >
            {m}
          </button>
        ))}
      </div>

      {mode==='add' && (
        <div className="space-y-2">
          <label className="block text-sm font-[560]" htmlFor="devices-add">Devices to add (space-separated)</label>
          <input id="devices-add" className="field font-mono" value={form.add} onChange={(e)=>setForm({ ...form, add: e.target.value })} />
        </div>
      )}
      {mode==='remove' && (
        <div className="space-y-2">
          <label className="block text-sm font-[560]">Devices to remove (space-separated)</label>
          <input className="field font-mono" value={form.remove} onChange={(e)=>setForm({ ...form, remove: e.target.value })} />
        </div>
      )}
      {mode==='replace' && (
        <div className="space-y-2">
          <div>
            <label className="block text-sm font-[560]">Old device</label>
            <input className="field font-mono" value={form.replaceOld} onChange={(e)=>setForm({ ...form, replaceOld: e.target.value })} />
          </div>
          <div>
            <label className="block text-sm font-[560]">New device</label>
            <input className="field font-mono" value={form.replaceNew} onChange={(e)=>setForm({ ...form, replaceNew: e.target.value })} />
          </div>
        </div>
      )}

      <div className="flex gap-2">
        <Button variant="outline" onClick={onPlan} disabled={!mode}>Show the plan</Button>
        <Button variant="destructive" onClick={onApply} disabled={!plan || (form.confirm.trim().toUpperCase() !== (mode==='add'?'ADD':mode==='remove'?'REMOVE':'REPLACE'))}>Apply the plan</Button>
      </div>

      {plan && (
        <div className="space-y-4">
          <ol className="divide-y divide-hairline border-y border-hairline">
            {(plan.steps||[]).map((s:any, i:number)=> (
              <li key={s.id||s.ID} className="grid grid-cols-[2rem_1fr] gap-x-3 py-3">
                <span className="font-mono text-sm tabular-nums text-ink-2">{i + 1}</span>
                <div className="space-y-1">
                  <p className="text-sm">{s.description||s.Description}</p>
                  <p className="break-all font-mono text-xs text-ink-2">{s.command||s.Command}</p>
                </div>
              </li>
            ))}
          </ol>
          {(plan.warnings||[]).map((w:string) => (
            <p key={w} className="flex items-start gap-2 text-sm">
              <WarningIcon className="mt-0.5 h-4 w-4 shrink-0 text-status-warning" />
              {w}
            </p>
          ))}
          <div className="space-y-2">
            <label htmlFor="device-confirm" className="block text-sm font-[560]">
              Type <span className="font-mono">{mode==='add'?'ADD':mode==='remove'?'REMOVE':'REPLACE'}</span> to apply the plan
            </label>
            <input id="device-confirm" className="field font-mono" autoComplete="off" value={form.confirm} onChange={(e)=>setForm({ ...form, confirm: e.target.value })} />
          </div>
        </div>
      )}

      {error && (
        <p role="alert" className="flex items-start gap-2 text-sm">
          <FailedIcon className="mt-0.5 h-4 w-4 shrink-0 text-status-failed" />
          {error}
        </p>
      )}

      {tx && (
        <div>
          <p className="mb-2 font-mono text-[0.8125rem] text-ink-2">Log</p>
          {/* A log that streams: not announced line by line. */}
          <pre aria-live="off" className="max-h-48 overflow-auto whitespace-pre-wrap border border-hairline p-3 font-mono text-xs">{log.join('\n')}</pre>
        </div>
      )}
    </section>
  )
}


