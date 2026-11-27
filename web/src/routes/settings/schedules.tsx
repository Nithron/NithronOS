import { useEffect, useMemo, useRef, useState } from 'react'
import { formatDateTime } from '@/lib/datetime'
import { getSchedules, updateSchedules, type Schedules } from '@/api/schedules'
import { useNavigate } from 'react-router-dom'

export default function SettingsSchedules() {
  const [sched, setSched] = useState<Schedules | null>(null)
  const [draft, setDraft] = useState<{ smartScan: string; btrfsScrub: string }>({ smartScan: '', btrfsScrub: '' })
  const [lastFstrim, setLastFstrim] = useState<string>('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<{ smartScan?: string; btrfsScrub?: string }>({})
  const nav = useNavigate()
  const smartRef = useRef<HTMLInputElement>(null)
  const scrubRef = useRef<HTMLInputElement>(null)

  useEffect(() => { getSchedules().then((s: any) => { setSched(s); setDraft({ smartScan: s.smartScan, btrfsScrub: s.btrfsScrub }); if (s.lastFstrim) setLastFstrim(s.lastFstrim) }).catch(() => {}) }, [])

  // In tests we avoid data-router-only blockers; simple window confirm in real app can be added at top-level

  const onChange = (field: 'smartScan'|'btrfsScrub', val: string) => {
    setDraft((d) => ({ ...d, [field]: val }))
    setFieldErrors((fe) => ({ ...fe, [field]: undefined }))
  }

  function validateField(value: string): string | undefined {
    if (!value || value.trim() === '') return 'Required'
    if (value.length > 120) return 'Must be 120 characters or fewer'
    if (!/\s/.test(value)) return 'Must include a space between parts (e.g., weekday and time)'
    return undefined
  }

  const helper = useMemo(() => ({
    smartScan: 'Examples: "Sun 03:00", "Wed 02:00"',
    btrfsScrub: 'Example: "Sun *-*-01..07 03:00" (first Sunday monthly)',
  }), [])

  async function onSave() {
    if (!draft) return
    setSaving(true)
    setError(null)
    // Client-side validation
    const clientErrors: { smartScan?: string; btrfsScrub?: string } = {
      smartScan: validateField(draft.smartScan),
      btrfsScrub: validateField(draft.btrfsScrub),
    }
    setFieldErrors(clientErrors)
    const firstClientErr = (clientErrors.smartScan ? 'smartScan' : clientErrors.btrfsScrub ? 'btrfsScrub' : '') as
      | 'smartScan'
      | 'btrfsScrub'
      | ''
    if (firstClientErr) {
      if (firstClientErr === 'smartScan') smartRef.current?.focus()
      else if (firstClientErr === 'btrfsScrub') scrubRef.current?.focus()
      setSaving(false)
      if ((globalThis as any).__DEV__) console.debug('[schedules] client validation failed', clientErrors)
      return
    }
    try {
      const res = await updateSchedules(draft)
      setSched(res)
      setFieldErrors({})
      try { const { toast } = await import('@/components/ui/toast'); toast.success('Schedules saved') } catch {}
    } catch (e: any) {
      if (e && typeof e === 'object' && (e.status === 400 || e.status === 422)) {
        setError(e?.message || 'Validation error')
        const details = (e.data && e.data.error && e.data.error.details) || {}
        const code = e.data && e.data.error && e.data.error.code
        const newFieldErrors: { smartScan?: string; btrfsScrub?: string } = {}
        if (code === 'schedule.invalid') {
          const field = details.field as 'smartScan' | 'btrfsScrub' | undefined
          const hint = details.hint as string | undefined
          if (field) newFieldErrors[field] = hint || 'Invalid schedule'
        }
        setFieldErrors((prev) => ({ ...prev, ...newFieldErrors }))
        const first = newFieldErrors.smartScan ? 'smartScan' : newFieldErrors.btrfsScrub ? 'btrfsScrub' : ''
        if (first === 'smartScan') smartRef.current?.focus()
        if (first === 'btrfsScrub') scrubRef.current?.focus()
        if ((globalThis as any).__DEV__) console.debug('[schedules] save 4xx', { status: e.status, body: e.data })
      } else {
        try { const { toast } = await import('@/components/ui/toast'); toast.error('Failed to save schedules') } catch {}
        if ((globalThis as any).__DEV__) console.debug('[schedules] save 5xx/other', e)
      }
    } finally {
      setSaving(false)
    }
  }

  function onRestoreDefaults() {
    setDraft({ smartScan: 'Sun 03:00', btrfsScrub: 'Sun *-*-01..07 03:00' })
  }

  return (
    <div className="space-y-4">
      <div>
        <h1 className="screen-title">Schedules</h1>
        <p className="text-sm text-muted-foreground">System maintenance cadences (systemd OnCalendar).</p>
      </div>
      <div className="border border-hairline p-4 space-y-3">
        <div>
          <label className="block text-sm mb-1">Smart scan (SMART)</label>
          <input ref={smartRef} name="smartScan" placeholder="Sun 03:00" className="w-full rounded border border-muted/30 bg-background px-2 py-1" value={draft.smartScan} onChange={(e) => onChange('smartScan', e.target.value)} aria-invalid={!!fieldErrors.smartScan} aria-describedby="smartScanHelp smartScanErr" />
          <p id="smartScanHelp" className="mt-1 text-xs text-muted-foreground">{helper.smartScan}</p>
          {fieldErrors.smartScan && <p id="smartScanErr" className="mt-1 text-xs text-status-failed-text">{fieldErrors.smartScan}</p>}
        </div>
        <div>
          <label className="block text-sm mb-1">Btrfs scrub</label>
          <input ref={scrubRef} name="btrfsScrub" placeholder="Sun *-*-01..07 03:00" className="w-full rounded border border-muted/30 bg-background px-2 py-1" value={draft.btrfsScrub} onChange={(e) => onChange('btrfsScrub', e.target.value)} aria-invalid={!!fieldErrors.btrfsScrub} aria-describedby="btrfsScrubHelp btrfsScrubErr" />
          <p id="btrfsScrubHelp" className="mt-1 text-xs text-muted-foreground">{helper.btrfsScrub}</p>
          {fieldErrors.btrfsScrub && <p id="btrfsScrubErr" className="mt-1 text-xs text-status-failed-text">{fieldErrors.btrfsScrub}</p>}
        </div>
        {error && <div className="border border-status-failed p-2 text-status-failed-text text-xs">{error}</div>}
        {lastFstrim && <div className="text-xs text-muted-foreground">Last fstrim: {formatDateTime(lastFstrim)}</div>}
        <div className="flex gap-2">
          <button className="rounded bg-primary px-3 py-1 text-sm text-background disabled:opacity-50" disabled={saving} onClick={onSave}>Save</button>
          <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={onRestoreDefaults}>Restore defaults</button>
          <button className="rounded border border-muted/30 px-3 py-1 text-sm" onClick={() => nav(-1)}>Cancel</button>
          {sched?.updatedAt && <span className="ml-auto text-xs text-muted-foreground">Last updated: {sched.updatedAt}</span>}
        </div>
      </div>
    </div>
  )
}


