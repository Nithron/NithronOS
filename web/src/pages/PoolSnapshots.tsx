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
      <div className="rounded bg-card p-3 space-y-2">
        <div className="grid gap-2 md:grid-cols-3">
          <input className="rounded bg-background p-2" placeholder="Subvolume path (e.g., /mnt/pool/data)" value={subvol} onChange={(e) => setSubvol(e.target.value)} />
          <input className="rounded bg-background p-2" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="btn bg-primary text-primary-foreground" onClick={createSnap}>Create snapshot</button>
        </div>
        {error && <div className="text-sm text-red-400">{error}</div>}
      </div>
      <table className="w-full text-sm">
        <thead className="text-left text-muted-foreground">
          <tr><th>Name</th><th>Path</th></tr>
        </thead>
        <tbody>
          {snaps.map((s) => (
            <tr key={s.path} className="border-t border-muted/20">
              <td className="py-2">{s.name}</td>
              <td className="font-mono text-xs">{s.path}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}


