import { useEffect, useState } from 'react'
import { storageApi } from '@/api/storage'
import { useNavigate } from 'react-router-dom'
import { getErrorMessage } from '@/lib/nos-client'

export function PoolsCreate() {
  const nav = useNavigate()
  const [step, setStep] = useState(1)
  const [label, setLabel] = useState('pool1')
  const [mountpoint, setMountpoint] = useState('/srv/pool/pool1')
  const [devices, setDevices] = useState<string[]>([])
  const [raidData, setRaidData] = useState<'single'|'raid1'|'raid10'>('raid1')
  const [raidMeta, setRaidMeta] = useState<'single'|'raid1'|'raid10'>('raid1')
  const [encrypt, setEncrypt] = useState<{ enabled: boolean; keyfile?: string }>({ enabled: false })
  const [diskList, setDiskList] = useState<any[]>([])
  const [plan, setPlan] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  const [disksLoaded, setDisksLoaded] = useState(false)
  useEffect(() => {
    // A failed disk list is shown, not thrown (it used to be an unhandled
    // rejection and an empty page).
    storageApi.disks()
      .then((d) => setDiskList(d))
      .catch((e: unknown) => setError(`The disks could not be listed: ${getErrorMessage(e)}`))
      .finally(() => setDisksLoaded(true))
  }, [])

  async function doPlan() {
    setError(null)
    try {
      const res = await storageApi.planCreate({ name: label, mountpoint, devices, raidData, raidMeta, encrypt })
      setPlan(res)
      setStep(5)
    } catch (e: any) {
      setError(e.message)
    }
  }

  async function doCreate() {
    setError(null)
    try {
      // The pool itself, as planned; nosd has the agent build and run the
      // commands (the plan's command lines are only shown).
      await storageApi.applyCreate({ name: label, mountpoint, devices, raidData, raidMeta, encrypt, mountOptions: plan?.mountOptions, confirm: 'CREATE' })
      const id = label
      nav(`/storage/${encodeURIComponent(id)}?mount=${encodeURIComponent(mountpoint)}`)
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <div className="space-y-4">
      <h1 className="screen-title">Create pool</h1>
      {error && <div className="text-sm text-red-400">{error}</div>}
      <div className="rounded-lg bg-card p-4 space-y-3">
        {step === 1 && (
          <div className="space-y-2">
            <div className="text-sm">Select devices <span className="text-red-500">(will be erased)</span></div>
            <div className="space-y-1">
              {disksLoaded && diskList.length === 0 && !error && (
                <div className="text-sm text-muted-foreground">No disks found.</div>
              )}
              {diskList.map((d) => (
                <label key={d.path || d.name} className="flex items-center gap-2">
                  <input type="checkbox" checked={devices.includes(d.path || d.name)} onChange={(e) => {
                    const p = d.path || d.name
                    setDevices((prev) => e.target.checked ? [...prev, p] : prev.filter((x) => x !== p))
                  }} />
                  <span>{d.name} <span className="text-muted-foreground">({formatBytes(Number(d.size))})</span></span>
                </label>
              ))}
            </div>
            <div className="flex gap-2"><button className="btn" onClick={() => setStep(2)} disabled={devices.length === 0}>Next</button></div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-2">
            <div>
              <label className="block text-sm mb-1">Name</label>
              <input className="w-full rounded bg-background p-2" value={label} onChange={(e) => { setLabel(e.target.value); setMountpoint(`/srv/pool/${e.target.value || 'pool'}`) }} />
            </div>
            <div>
              <label className="block text-sm mb-1">Mountpoint</label>
              <input className="w-full rounded bg-background p-2" value={mountpoint} onChange={(e) => setMountpoint(e.target.value)} />
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              <div>
                <label className="block text-sm mb-1">Data profile</label>
                <select className="w-full rounded bg-background p-2" value={raidData} onChange={(e) => setRaidData(e.target.value as any)}>
                  <option value="single">single</option>
                  <option value="raid1">raid1</option>
                  <option value="raid10">raid10</option>
                </select>
              </div>
              <div>
                <label className="block text-sm mb-1">Metadata profile</label>
                <select className="w-full rounded bg-background p-2" value={raidMeta} onChange={(e) => setRaidMeta(e.target.value as any)}>
                  <option value="single">single</option>
                  <option value="raid1">raid1</option>
                  <option value="raid10">raid10</option>
                </select>
              </div>
            </div>
            <div className="flex gap-2"><button className="btn" onClick={() => setStep(3)}>Next</button></div>
          </div>
        )}

        {step === 3 && (
          <div className="space-y-2">
            <label className="inline-flex items-center gap-2">
              <input type="checkbox" checked={encrypt.enabled} onChange={(e) => setEncrypt({ ...encrypt, enabled: e.target.checked })} />
              <span>Encrypt devices with LUKS</span>
            </label>
            {encrypt.enabled && (
              <div>
                <label className="block text-sm mb-1">Keyfile</label>
                <input className="w-full rounded bg-background p-2" value={encrypt.keyfile || ''} onChange={(e) => setEncrypt({ ...encrypt, keyfile: e.target.value })} placeholder={`/etc/nos/keys/${label}.key`} />
                <div className="mt-1 text-xs text-muted-foreground">Keyfile is stored on disk; consider adding a passphrase slot after creation.</div>
              </div>
            )}
            <div className="flex gap-2"><button className="btn" onClick={() => setStep(4)}>Next</button></div>
          </div>
        )}

        {step === 4 && (
          <div className="space-y-2">
            <button className="btn btn-primary" onClick={doPlan}>Fetch plan</button>
          </div>
        )}

        {step === 5 && (
          <div className="space-y-2">
            <h2 className="mb-2 text-lg font-medium">Plan</h2>
            <pre className="text-xs text-muted-foreground whitespace-pre-wrap">{JSON.stringify(plan?.plan || plan, null, 2)}</pre>
            <div>
              <label className="block text-sm mb-1">Type CREATE to confirm</label>
              <input className="w-full rounded bg-background p-2" onChange={() => {}} placeholder="CREATE" />
            </div>
            <div className="flex gap-2">
              <button className="btn" onClick={doCreate}>Create</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function formatBytes(n: number): string {
  if (!n || Number.isNaN(n)) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) { v/=1024; i++ }
  return `${v.toFixed(1)} ${units[i]}`
}


