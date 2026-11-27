import { useEffect, useState } from 'react'
import http from '@/lib/nos-client'

export function PoolSnapshots({ id }: { id: string }) {
  const [snaps, setSnaps] = useState<any[]>([])
  const [name, setName] = useState('snap-' + new Date().toISOString().slice(0,19).replace(/[:T]/g,'-'))
  const [subvol, setSubvol] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    refresh()
  }, [id])

  async function refresh() {
    const j = await http.pools.snapshots(id) as any[]
    setSnaps(j)
  }

  async function createSnap() {
    setError(null)
    try {
      await http.post(`/v1/pools/${id}/snapshots`, { subvol, name })
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <div className="grid gap-2 md:grid-cols-3">
          <input className="field font-mono" aria-label="Subvolume path" placeholder="Subvolume path, for example /srv/pool/pool1/data" value={subvol} onChange={(e) => setSubvol(e.target.value)} />
          <input className="field font-mono" aria-label="Snapshot name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="btn btn-primary" onClick={createSnap}>Create snapshot</button>
        </div>
        {error && <p role="alert" className="text-sm text-status-failed-text">{error}</p>}
      </div>
      <table className="w-full text-sm">
        <thead className="text-left font-mono text-[0.8125rem] text-ink-2">
          <tr className="border-b border-hairline"><th className="py-2 font-normal">Name</th><th className="py-2 font-normal">Path</th></tr>
        </thead>
        <tbody>
          {snaps.map((s) => (
            <tr key={s.path} className="border-b border-hairline">
              <td className="py-3 font-mono">{s.name}</td>
              <td className="font-mono text-xs">{s.path}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}


