import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { storageApi } from '@/api/storage'
import { getErrorMessage } from '@/lib/nos-client'
import { Button } from '@/components/ui/button'
import { Mark, StatusMark, Working } from '@/components/ui/pixel'
import { FailedIcon, WarningIcon } from '@/components/icons'
import { cn } from '@/lib/utils'

type Profile = 'single' | 'raid1' | 'raid10'

interface PlanStep {
  id: string
  description: string
  command: string
  destructive: boolean
}

interface Plan {
  plan?: { steps?: PlanStep[] }
  fstab?: string[]
  warnings?: string[]
  mountOptions?: string
}

interface Disk {
  name: string
  path: string
  size: number
  model?: string
}

const steps = ['Disks', 'Name and profile', 'Encryption', 'Plan'] as const

const profiles: Record<Profile, string> = {
  single: 'single: no copy, all space usable',
  raid1: 'raid1: two copies on different disks',
  raid10: 'raid10: striped mirrors, four disks or more',
}

function formatBytes(n: number): string {
  if (!n || Number.isNaN(n)) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}

/**
 * Creating a pool: choose disks, a name and profile, encryption, then the
 * plan the agent would run. Nothing is erased until the pool's name is
 * typed under the plan.
 */
export function PoolsCreate() {
  const nav = useNavigate()
  const [step, setStep] = useState(0)
  const [label, setLabel] = useState('pool1')
  const [mountpoint, setMountpoint] = useState('/srv/pool/pool1')
  const [devices, setDevices] = useState<string[]>([])
  const [raidData, setRaidData] = useState<Profile>('raid1')
  const [raidMeta, setRaidMeta] = useState<Profile>('raid1')
  const [encrypted, setEncrypted] = useState(false)
  const [diskList, setDiskList] = useState<Disk[]>([])
  const [disksLoaded, setDisksLoaded] = useState(false)
  const [plan, setPlan] = useState<Plan | null>(null)
  const [planning, setPlanning] = useState(false)
  const [creating, setCreating] = useState(false)
  const [typed, setTyped] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    storageApi
      .disks()
      .then((d) => setDiskList(d))
      .catch((e: unknown) => setError(`The disks could not be listed: ${getErrorMessage(e)}`))
      .finally(() => setDisksLoaded(true))
  }, [])

  const spec = { name: label, mountpoint, devices, raidData, raidMeta, encrypt: { enabled: encrypted } }

  async function doPlan() {
    setError(null)
    setPlanning(true)
    try {
      setPlan((await storageApi.planCreate(spec)) as Plan)
      setTyped('')
      setStep(3)
    } catch (e: unknown) {
      setError(getErrorMessage(e))
    } finally {
      setPlanning(false)
    }
  }

  async function doCreate() {
    setError(null)
    setCreating(true)
    try {
      // The pool itself, as planned; nosd has the agent build and run the
      // commands (the plan's command lines are only shown).
      await storageApi.applyCreate({ ...spec, mountOptions: plan?.mountOptions, confirm: 'CREATE' })
      nav(`/storage/${encodeURIComponent(label)}?mount=${encodeURIComponent(mountpoint)}`)
    } catch (e: unknown) {
      setError(getErrorMessage(e))
      setCreating(false)
    }
  }

  const planSteps = plan?.plan?.steps ?? []
  const confirmed = typed === label && label !== ''

  return (
    <div className="max-w-3xl space-y-8">
      <h1 className="screen-title">Create pool</h1>

      <ol className="flex flex-wrap gap-x-6 gap-y-2 border-b border-hairline pb-3 text-sm">
        {steps.map((name, i) => (
          <li key={name} aria-current={i === step ? 'step' : undefined}>
            <StatusMark state={i < step ? 'ok' : i === step ? 'running' : 'off'} className={i === step ? 'font-[560] text-ink' : 'text-ink-2'}>
              {name}
            </StatusMark>
          </li>
        ))}
      </ol>

      {error && (
        <p role="alert" className="flex items-start gap-2 text-sm text-ink">
          <FailedIcon className="mt-0.5 h-4 w-4 shrink-0 text-status-failed" />
          {error}
        </p>
      )}

      {step === 0 && (
        <section className="space-y-4">
          <p className="text-sm">Choose the disks for the pool. Everything on them is erased when the pool is created.</p>
          {!disksLoaded && <Working>Listing the disks</Working>}
          {disksLoaded && diskList.length === 0 && !error && <p className="text-sm text-ink-2">No disk was found.</p>}
          {diskList.length > 0 && (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-hairline font-mono text-[0.8125rem] text-ink-2">
                  <th className="w-8 py-2" />
                  <th className="py-2 text-left font-normal">Disk</th>
                  <th className="py-2 text-left font-normal">Model</th>
                  <th className="py-2 text-right font-normal">Size</th>
                </tr>
              </thead>
              <tbody>
                {diskList.map((d) => {
                  const p = d.path || d.name
                  return (
                    <tr key={p} className="border-b border-hairline">
                      <td className="py-3">
                        <input
                          id={`disk-${d.name}`}
                          type="checkbox"
                          checked={devices.includes(p)}
                          onChange={(e) => setDevices((prev) => (e.target.checked ? [...prev, p] : prev.filter((x) => x !== p)))}
                        />
                      </td>
                      <td className="py-3">
                        <label htmlFor={`disk-${d.name}`} className="cursor-pointer font-mono">
                          {p}
                        </label>
                      </td>
                      <td className="py-3 text-ink-2">{d.model || '-'}</td>
                      <td className="py-3 text-right font-mono tabular-nums">{formatBytes(Number(d.size))}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
          <Button onClick={() => setStep(1)} disabled={devices.length === 0}>
            Next
          </Button>
        </section>
      )}

      {step === 1 && (
        <section className="space-y-5">
          <div className="space-y-2">
            <label htmlFor="pool-name" className="block text-sm font-[560]">Name</label>
            <input
              id="pool-name"
              className="field font-mono"
              value={label}
              onChange={(e) => {
                setLabel(e.target.value)
                setMountpoint(`/srv/pool/${e.target.value || 'pool'}`)
              }}
            />
          </div>
          <div className="space-y-2">
            <label htmlFor="pool-mount" className="block text-sm font-[560]">Mount point</label>
            <input id="pool-mount" className="field font-mono" value={mountpoint} onChange={(e) => setMountpoint(e.target.value)} />
            <p className="text-sm text-ink-2">Under /srv or /mnt.</p>
          </div>
          {(['data', 'metadata'] as const).map((kind) => (
            <div key={kind} className="space-y-2">
              <label htmlFor={`profile-${kind}`} className="block text-sm font-[560]">
                {kind === 'data' ? 'Data profile' : 'Metadata profile'}
              </label>
              <select
                id={`profile-${kind}`}
                className="field"
                value={kind === 'data' ? raidData : raidMeta}
                onChange={(e) => (kind === 'data' ? setRaidData : setRaidMeta)(e.target.value as Profile)}
              >
                {Object.entries(profiles).map(([value, text]) => (
                  <option key={value} value={value}>
                    {text}
                  </option>
                ))}
              </select>
            </div>
          ))}
          <div className="flex gap-3">
            <Button variant="outline" onClick={() => setStep(0)}>Back</Button>
            <Button onClick={() => setStep(2)} disabled={!label || !mountpoint}>Next</Button>
          </div>
        </section>
      )}

      {step === 2 && (
        <section className="space-y-5">
          <label className="flex items-center gap-3 text-sm">
            <input type="checkbox" checked={encrypted} onChange={(e) => setEncrypted(e.target.checked)} />
            Encrypt the disks (LUKS2)
          </label>
          {encrypted && (
            <p className="text-sm text-ink-2">
              The key is made on this system at <span className="font-mono text-ink">/etc/nos/keys/{label}.key</span> and
              opens the disks at boot. Keep a copy of it somewhere else: without it the data cannot be read.
            </p>
          )}
          <div className="flex gap-3">
            <Button variant="outline" onClick={() => setStep(1)}>Back</Button>
            <Button onClick={doPlan} disabled={planning}>
              {planning ? 'Planning' : 'Show the plan'}
            </Button>
          </div>
        </section>
      )}

      {step === 3 && plan && (
        <section className="space-y-6">
          <p className="text-sm">
            What will run, in order. Steps that erase a disk say so.
          </p>
          <ol className="divide-y divide-hairline border-y border-hairline">
            {planSteps.map((s, i) => (
              <li key={s.id} className="grid grid-cols-[2rem_1fr] gap-x-3 py-3">
                <span className="font-mono text-sm tabular-nums text-ink-2">{i + 1}</span>
                <div className="space-y-1">
                  <p className={cn('text-sm', s.destructive && 'font-[560]')}>
                    {s.description}
                    {s.destructive && ' (erases data)'}
                  </p>
                  <p className="break-all font-mono text-xs text-ink-2">{s.command}</p>
                </div>
              </li>
            ))}
          </ol>
          {(plan.fstab ?? []).filter(Boolean).length > 0 && (
            <div className="space-y-1">
              <p className="font-mono text-[0.8125rem] text-ink-2">Line added to /etc/fstab</p>
              {(plan.fstab ?? []).filter(Boolean).map((line) => (
                <p key={line} className="break-all font-mono text-xs">{line}</p>
              ))}
            </div>
          )}
          {(plan.warnings ?? []).length > 0 && (
            <ul className="space-y-1">
              {(plan.warnings ?? []).map((w) => (
                <li key={w} className="flex items-start gap-2 text-sm">
                  <WarningIcon className="mt-0.5 h-4 w-4 shrink-0 text-status-warning" />
                  {w}
                </li>
              ))}
            </ul>
          )}
          <div className="space-y-2">
            <label htmlFor="confirm-name" className="block text-sm font-[560]">
              Type <span className="font-mono">{label}</span> to erase {devices.length} disk{devices.length === 1 ? '' : 's'} and create the pool
            </label>
            <input
              id="confirm-name"
              className="field font-mono"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
            />
          </div>
          <div className="flex items-center gap-3">
            <Button variant="outline" onClick={() => setStep(2)} disabled={creating}>Back</Button>
            <Button variant="destructive" onClick={doCreate} disabled={!confirmed || creating}>
              {creating ? 'Creating' : `Erase ${devices.length} disk${devices.length === 1 ? '' : 's'} and create ${label}`}
            </Button>
            {creating && <Mark state="running" />}
          </div>
        </section>
      )}
    </div>
  )
}
