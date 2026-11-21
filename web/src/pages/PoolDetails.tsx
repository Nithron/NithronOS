import { useEffect, useMemo, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { PoolSnapshots } from './PoolSnapshots'
import http, { api } from '@/lib/nos-client'
import type { TxLogResponse } from '@/types/api'

export function PoolDetails() {
  const { id } = useParams()
  const [pools, setPools] = useState<any[]>([])
  const pool = useMemo(() => pools.find((p) => p.id === id || p.mount === id || p.uuid === id), [pools, id])
  const [tab, setTab] = useState<'overview'|'snapshots'|'devices'>('overview')

  useEffect(() => {
    http.pools.listUnversioned().then((data: any) => setPools(data))
  }, [])

  if (!id) return <div className="p-4">Missing id</div>

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Pool Details</h1>
        <Link to="/storage" className="text-sm text-primary">Back to Storage</Link>
      </div>
      <div className="flex gap-3 border-b border-muted/30">
        <button className={`px-2 py-1 text-sm ${tab==='overview'?'text-primary border-b-2 border-primary':''}`} onClick={()=>setTab('overview')}>Overview</button>
        <button className={`px-2 py-1 text-sm ${tab==='snapshots'?'text-primary border-b-2 border-primary':''}`} onClick={()=>setTab('snapshots')}>Snapshots</button>
        <button className={`px-2 py-1 text-sm ${tab==='devices'?'text-primary border-b-2 border-primary':''}`} onClick={()=>setTab('devices')}>Devices</button>
      </div>

      {tab==='overview' && (
        pool ? (
          <div className="rounded-lg bg-card p-4 space-y-2">
            <div className="grid gap-2 md:grid-cols-3">
              <Field label="Label" value={pool.label || '-'} />
              <Field label="UUID" value={pool.uuid || '-'} mono />
              <Field label="RAID" value={(pool.raid || '-').toUpperCase()} />
              <Field label="Mount" value={pool.mount || pool.id} mono />
              <Field label="Size" value={formatBytes(pool.size)} />
              <Field label="Used / Free" value={`${formatBytes(pool.used)} / ${formatBytes(pool.free)}`} />
            </div>
          </div>
        ) : (<div className="text-sm text-muted-foreground">Loading...</div>)
      )}

      {tab==='overview' && pool && (
        <MountOptionsCard pool={pool} id={id!} onSaved={() => { /* no refetch needed here */ }} />
      )}

      {tab==='snapshots' && (
        <section className="space-y-3">
          <h2 className="text-lg font-medium">Snapshots</h2>
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
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={mono ? 'font-mono text-xs' : ''}>{value}</div>
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
        <h2 className="text-lg font-medium">Mount options</h2>
        <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={onEdit}>Edit</button>
      </div>
      <div className="flex items-center gap-2">
        <span className="inline-flex items-center rounded-full bg-secondary px-2 py-0.5 text-xs text-secondary-foreground font-mono">{current || '...'}</span>
      </div>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-lg rounded-lg bg-card p-4 shadow">
            <h3 className="text-md font-medium mb-2">Edit mount options</h3>
            <textarea className="w-full font-mono text-sm rounded border border-muted/30 bg-background p-2 h-28" value={value} onChange={(e)=>{ setValue(e.target.value); setError(null) }} />
            <p className="mt-1 text-xs text-muted-foreground">Examples: SSD: compress=zstd:3,ssd,discard=async,noatime • HDD/Mixed: compress=zstd:3,noatime</p>
            {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
            <div className="mt-3 flex items-center gap-2">
              <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={() => setValue(computeDefaultFromPoolDevices(pool))}>Restore defaults</button>
              <div className="flex-1" />
              <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={() => setOpen(false)} disabled={saving}>Cancel</button>
              <button className="rounded bg-primary px-3 py-1 text-sm text-background disabled:opacity-50" onClick={onSave} disabled={saving}>Save</button>
            </div>
          </div>
        </div>
      )}
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
      <h2 className="text-lg font-medium">Devices</h2>
      <div className="flex gap-2">
        <button className={`rounded border border-muted/30 px-3 py-1 text-sm ${mode==='add'?'bg-secondary':''}`} onClick={()=>{ setMode('add'); setPlan(null); setLog([]); setTx('') }}>Add</button>
        <button className={`rounded border border-muted/30 px-3 py-1 text-sm ${mode==='remove'?'bg-secondary':''}`} onClick={()=>{ setMode('remove'); setPlan(null); setLog([]); setTx('') }}>Remove</button>
        <button className={`rounded border border-muted/30 px-3 py-1 text-sm ${mode==='replace'?'bg-secondary':''}`} onClick={()=>{ setMode('replace'); setPlan(null); setLog([]); setTx('') }}>Replace</button>
      </div>

      {mode==='add' && (
        <div className="rounded bg-card p-3 space-y-2">
          <label className="text-xs" htmlFor="devices-add">Devices to add (space-separated)</label>
          <input id="devices-add" className="w-full rounded border border-muted/30 bg-background px-2 py-1 font-mono text-xs" value={form.add} onChange={(e)=>setForm({ ...form, add: e.target.value })} />
        </div>
      )}
      {mode==='remove' && (
        <div className="rounded bg-card p-3 space-y-2">
          <label className="text-xs">Devices to remove (space-separated)</label>
          <input className="w-full rounded border border-muted/30 bg-background px-2 py-1 font-mono text-xs" value={form.remove} onChange={(e)=>setForm({ ...form, remove: e.target.value })} />
        </div>
      )}
      {mode==='replace' && (
        <div className="rounded bg-card p-3 space-y-2">
          <div>
            <label className="text-xs">Old device</label>
            <input className="w-full rounded border border-muted/30 bg-background px-2 py-1 font-mono text-xs" value={form.replaceOld} onChange={(e)=>setForm({ ...form, replaceOld: e.target.value })} />
          </div>
          <div>
            <label className="text-xs">New device</label>
            <input className="w-full rounded border border-muted/30 bg-background px-2 py-1 font-mono text-xs" value={form.replaceNew} onChange={(e)=>setForm({ ...form, replaceNew: e.target.value })} />
          </div>
        </div>
      )}

      <div className="flex gap-2">
        <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={onPlan} disabled={!mode}>Plan</button>
        <button className="rounded bg-primary px-3 py-1 text-sm text-background disabled:opacity-50" onClick={onApply} disabled={!plan || (form.confirm.trim().toUpperCase() !== (mode==='add'?'ADD':mode==='remove'?'REMOVE':'REPLACE'))}>Apply</button>
      </div>

      {plan && (
        <div className="rounded bg-card p-3">
          <div className="text-sm font-medium mb-2">Plan</div>
          <ul className="list-disc pl-5 text-xs">
            {(plan.steps||[]).map((s:any)=> (<li key={s.id||s.ID}>{s.description||s.Description} <span className="font-mono">{s.command||s.Command}</span></li>))}
          </ul>
          {(plan.warnings||[]).length>0 && (
            <div className="mt-2 text-xs text-yellow-600">Warnings: {(plan.warnings||[]).join('; ')}</div>
          )}
          <div className="mt-3 text-xs text-muted-foreground">Confirm code required: {mode==='add'?'ADD':mode==='remove'?'REMOVE':'REPLACE'}</div>
          <div className="mt-2">
            <label className="text-xs">Type the confirm code to enable Apply</label>
            <input aria-label="Confirm code" placeholder={(mode==='add'?'ADD':mode==='remove'?'REMOVE':'REPLACE')} className="mt-1 w-full rounded border border-muted/30 bg-background px-2 py-1 font-mono text-xs" value={form.confirm} onChange={(e)=>setForm({ ...form, confirm: e.target.value })} />
          </div>
        </div>
      )}

      {error && <div className="text-xs text-red-600">{error}</div>}

      {tx && (
        <div className="rounded bg-card p-3">
          <div className="text-sm font-medium mb-2">Transaction Log</div>
          <pre className="whitespace-pre-wrap text-xs font-mono max-h-48 overflow-auto">{log.join('\n')}</pre>
        </div>
      )}
    </section>
  )
}


