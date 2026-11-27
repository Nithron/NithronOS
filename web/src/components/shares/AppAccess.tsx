import { useQuery } from '@tanstack/react-query'
import { AppsIcon } from '@/components/icons'
import { Card } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { appsApi } from '@/api/apps'

interface AppAccessProps {
  selected: string[]
  onChange: (apps: string[]) => void
}

interface AppRow {
  id: string
  name: string
  installed: boolean
}

// rows lists the installed apps, plus any app the share is still open to
// that is no longer installed (so it can be switched off).
function rows(installed: Array<{ id: string; name: string }>, selected: string[]): AppRow[] {
  const list: AppRow[] = installed.map((a) => ({ id: a.id, name: a.name || a.id, installed: true }))
  for (const id of selected) {
    if (!list.some((r) => r.id === id)) list.push({ id, name: id, installed: false })
  }
  return list
}

/**
 * AppAccess opens a share to installed apps, such as an indexer. An app gets
 * read-only access through its own group, separate from the people in
 * Owners and Readers; it is off for every app until switched on here.
 */
export function AppAccess({ selected, onChange }: AppAccessProps) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['apps', 'installed'],
    queryFn: async () => (await appsApi.getInstalledApps()).items ?? [],
    staleTime: 10_000,
  })
  const list = rows(data ?? [], selected)

  const toggle = (id: string, on: boolean) =>
    onChange(on ? [...selected.filter((a) => a !== id), id] : selected.filter((a) => a !== id))

  return (
    <Card className="p-6 space-y-4">
      <div className="flex items-start gap-3">
        <AppsIcon className="h-5 w-5 mt-0.5 text-muted-foreground" aria-hidden />
        <div>
          <h3 className="font-medium">Apps that can read this share</h3>
          <p className="text-sm text-muted-foreground">
            An app switched on here can read every file in the share, but never change anything.
          </p>
        </div>
      </div>
      {isLoading && <p className="text-sm text-muted-foreground">Loading installed apps…</p>}
      {isError && <p className="text-sm text-destructive">Installed apps could not be loaded.</p>}
      {!isLoading && !isError && list.length === 0 && (
        <p className="text-sm text-muted-foreground">No apps are installed.</p>
      )}
      <ul className="space-y-3">
        {list.map((app) => (
          <li key={app.id} className="flex items-center justify-between gap-4">
            <Label htmlFor={`app-access-${app.id}`} className="flex flex-col">
              <span>{app.name}</span>
              {!app.installed && (
                <span className="text-xs text-muted-foreground">Not installed</span>
              )}
            </Label>
            <Switch
              id={`app-access-${app.id}`}
              checked={selected.includes(app.id)}
              onCheckedChange={(on: boolean) => toggle(app.id, on)}
            />
          </li>
        ))}
      </ul>
    </Card>
  )
}
