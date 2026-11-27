import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AccountIcon, ChevronRightIcon, DownloadIcon, LockIcon, NetworkIcon, NoticeIcon, ScheduleIcon, ServerIcon, TimeIcon, UsersIcon } from '@/components/icons'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { toast } from '@/components/ui/toast'
import { cn } from '@/lib/utils'
import { useAuth } from '@/lib/auth'
import { getErrorMessage } from '@/lib/nos-client'
import { passwordProblem, usersApi } from '@/api/users'
import {
  HOSTNAME_PATTERN,
  useAbout,
  useHostname,
  useNTP,
  useSystemMutations,
  useTimezone,
  useTimezoneList,
} from '@/api/system'
import { useAppearance, useSetTheme } from '@/api/appearance'
import { isThemeChoice, themeChoices, type ThemeChoice } from '@/lib/theme'

type Section = 'system' | 'account' | 'about'

const sections: Array<{ id: Section; label: string; icon: typeof ServerIcon }> = [
  { id: 'system', label: 'System', icon: ServerIcon },
  { id: 'account', label: 'My account', icon: AccountIcon },
  { id: 'about', label: 'About', icon: NoticeIcon },
]

const otherPages = [
  { label: 'Users', path: '/settings/users', icon: UsersIcon, adminOnly: true },
  { label: 'Groups', path: '/settings/groups', icon: UsersIcon, adminOnly: true },
  { label: 'Network', path: '/settings/network', icon: NetworkIcon, adminOnly: true },
  { label: 'Schedules', path: '/settings/schedules', icon: ScheduleIcon, adminOnly: true },
  { label: 'Updates', path: '/settings/updates', icon: DownloadIcon, adminOnly: true },
  { label: 'Two-factor sign-in', path: '/settings/2fa', icon: LockIcon, adminOnly: false },
]

function useIsAdmin(): boolean {
  const { session } = useAuth()
  return Boolean(session?.user.isAdmin || session?.user.roles?.includes('admin'))
}

function HostnameCard({ canEdit }: { canEdit: boolean }) {
  const { data, isLoading, error, refetch } = useHostname()
  const { setHostname } = useSystemMutations()
  const [value, setValue] = useState('')
  useEffect(() => setValue(data?.hostname ?? ''), [data?.hostname])
  const valid = HOSTNAME_PATTERN.test(value)
  const save = () =>
    setHostname.mutate(value, {
      onSuccess: () => toast.success(`The hostname is now ${value}`),
      onError: (e) => toast.error(getErrorMessage(e)),
    })
  return (
    <Card
      title="Name on the network"
      isLoading={isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => refetch()}
    >
      <div className="space-y-2">
        <Label htmlFor="hostname">Hostname</Label>
        <div className="flex gap-2">
          <Input
            id="hostname"
            value={value}
            disabled={!canEdit}
            onChange={(e) => setValue(e.target.value.trim())}
          />
          {canEdit && (
            <Button
              onClick={save}
              disabled={!valid || value === data?.hostname || setHostname.isPending}
            >
              {setHostname.isPending ? 'Saving…' : 'Save'}
            </Button>
          )}
        </div>
        <p
          className={value && !valid ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}
        >
          Letters, digits and '-'; computers on your network find nOS by this name.
        </p>
      </div>
    </Card>
  )
}

function TimeCard({ canEdit }: { canEdit: boolean }) {
  const tz = useTimezone()
  const list = useTimezoneList()
  const ntp = useNTP()
  const { setTimezone, setNTP } = useSystemMutations()
  const [zone, setZone] = useState('')
  const [servers, setServers] = useState('')
  useEffect(() => setZone(tz.data?.timezone ?? ''), [tz.data?.timezone])
  useEffect(() => setServers((ntp.data?.servers ?? []).join(' ')), [ntp.data?.servers])

  const saveZone = () =>
    setTimezone.mutate(zone, {
      onSuccess: () => toast.success(`The time zone is now ${zone}`),
      onError: (e) => toast.error(getErrorMessage(e)),
    })
  const saveNTP = (enabled: boolean) =>
    setNTP.mutate(
      { enabled, servers: servers.split(/[\s,]+/).filter(Boolean) },
      {
        onSuccess: () =>
          toast.success(enabled ? 'The clock is set from the internet' : 'Automatic time is off'),
        onError: (e) => toast.error(getErrorMessage(e)),
      },
    )
  const error = tz.error ?? ntp.error

  return (
    <Card
      title="Date and time"
      isLoading={tz.isLoading || ntp.isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => {
        tz.refetch()
        ntp.refetch()
      }}
    >
      <div className="space-y-6">
        <div className="space-y-2">
          <Label htmlFor="timezone">Time zone</Label>
          <div className="flex gap-2">
            <select
              id="timezone"
              className="h-10 flex-1 rounded-md border border-input bg-background px-3 text-sm"
              value={zone}
              disabled={!canEdit}
              onChange={(e) => setZone(e.target.value)}
            >
              {!list.data?.timezones.includes(zone) && zone && <option value={zone}>{zone}</option>}
              {(list.data?.timezones ?? []).map((z) => (
                <option key={z} value={z}>
                  {z}
                </option>
              ))}
            </select>
            {canEdit && (
              <Button
                onClick={saveZone}
                disabled={!zone || zone === tz.data?.timezone || setTimezone.isPending}
              >
                {setTimezone.isPending ? 'Saving…' : 'Save'}
              </Button>
            )}
          </div>
          {tz.data && (
            <p className="text-sm text-muted-foreground">
              <TimeIcon className="mr-1 inline h-3.5 w-3.5" aria-hidden />
              Time on nOS when loaded: {new Date(tz.data.time).toLocaleString()}
            </p>
          )}
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              <Label htmlFor="ntp">Set the time automatically</Label>
              <p className="text-sm text-muted-foreground">
                {ntp.data?.status === 'synchronized'
                  ? 'In sync with the time servers'
                  : ntp.data?.enabled
                    ? 'Not in sync yet'
                    : 'Off'}
              </p>
            </div>
            <Switch
              id="ntp"
              checked={ntp.data?.enabled ?? false}
              disabled={!canEdit || setNTP.isPending}
              onCheckedChange={saveNTP}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="ntp-servers">Time servers</Label>
            <div className="flex gap-2">
              <Input
                id="ntp-servers"
                placeholder="Debian's default servers"
                value={servers}
                disabled={!canEdit}
                onChange={(e) => setServers(e.target.value)}
              />
              {canEdit && (
                <Button
                  variant="outline"
                  onClick={() => saveNTP(ntp.data?.enabled ?? true)}
                  disabled={setNTP.isPending || servers === (ntp.data?.servers ?? []).join(' ')}
                >
                  Save
                </Button>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              Separated by spaces. Empty uses Debian's default servers.
            </p>
          </div>
        </div>
      </div>
    </Card>
  )
}

function ThemeCard({ canEdit }: { canEdit: boolean }) {
  const { data, isLoading, error, refetch } = useAppearance()
  const setTheme = useSetTheme()
  const current: ThemeChoice = isThemeChoice(data?.theme) ? data.theme : 'auto'
  const choose = (theme: ThemeChoice) =>
    setTheme.mutate(theme, {
      onSuccess: () => toast.success(theme === 'auto' ? 'The theme follows each device' : `The theme is ${theme}`),
      onError: (e) => toast.error(getErrorMessage(e)),
    })
  return (
    <Card
      title="Theme"
      description="Light or dark for everyone who signs in. Follow the system uses each device's own setting."
      isLoading={isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => refetch()}
    >
      <fieldset disabled={!canEdit || setTheme.isPending}>
        <legend className="sr-only">Theme</legend>
        <div className="inline-flex divide-x divide-rule overflow-hidden rounded-lg border border-rule">
          {themeChoices.map((c) => (
            <label
              key={c.value}
              className={cn(
                'flex min-h-11 cursor-pointer items-center px-4 text-sm transition-colors has-[:focus-visible]:outline has-[:focus-visible]:outline-2 has-[:focus-visible]:-outline-offset-4 has-[:focus-visible]:outline-ink',
                current === c.value ? 'bg-ink text-page' : 'hover:bg-ink/[.09]',
              )}
            >
              <input
                type="radio"
                name="theme"
                value={c.value}
                checked={current === c.value}
                onChange={() => choose(c.value)}
                className="sr-only"
              />
              {c.label}
            </label>
          ))}
        </div>
      </fieldset>
    </Card>
  )
}

function SystemSection() {
  const isAdmin = useIsAdmin()
  return (
    <div className="space-y-6">
      {!isAdmin && (
        <p className="text-sm text-muted-foreground">Only administrators can change these.</p>
      )}
      <HostnameCard canEdit={isAdmin} />
      <TimeCard canEdit={isAdmin} />
      <ThemeCard canEdit={isAdmin} />
    </div>
  )
}

function AccountSection() {
  const { session } = useAuth()
  const navigate = useNavigate()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [saving, setSaving] = useState(false)
  const problem = next ? passwordProblem(next) : null
  const ready = current !== '' && passwordProblem(next) === null && next === confirm

  const save = async () => {
    setSaving(true)
    try {
      await usersApi.changeOwnPassword(current, next)
      toast.success('Password changed; your other devices were signed out')
      setCurrent('')
      setNext('')
      setConfirm('')
    } catch (e) {
      toast.error(getErrorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <Card
        title="Password"
        description={`Signed in as ${session?.user.username ?? ''}. The same password opens the file shares.`}
      >
        <div className="max-w-md space-y-4">
          <div className="space-y-2">
            <Label htmlFor="current-password">Current password</Label>
            <Input
              id="current-password"
              type="password"
              autoComplete="current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">New password</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
            />
            {problem && <p className="text-sm text-destructive">{problem}</p>}
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-password">Repeat the new password</Label>
            <Input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
            />
            {confirm && confirm !== next && (
              <p className="text-sm text-destructive">The passwords don't match</p>
            )}
          </div>
          <Button onClick={save} disabled={!ready || saving}>
            {saving ? 'Saving…' : 'Change password'}
          </Button>
        </div>
      </Card>
      <Card
        title="Two-factor sign-in"
        description="A code from an authenticator app on top of your password."
      >
        <Button variant="outline" onClick={() => navigate('/settings/2fa')}>
          <LockIcon className="mr-2 h-4 w-4" aria-hidden />
          Manage two-factor sign-in
        </Button>
      </Card>
    </div>
  )
}

function formatBytes(n: number): string {
  if (!n) return 'Unknown'
  const gib = n / 1024 ** 3
  return gib >= 1 ? `${gib.toFixed(1)} GiB` : `${Math.round(n / 1024 ** 2)} MiB`
}

function formatUptime(seconds: number): string {
  if (!seconds) return 'Unknown'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  return d > 0 ? `${d} d ${h} h` : `${h} h ${Math.floor((seconds % 3600) / 60)} min`
}

function AboutSection() {
  const { data, isLoading, error, refetch } = useAbout()
  const rows: Array<[string, string]> = data
    ? [
        ['nOS', data.software.nos_version],
        ['Web interface', data.software.webui_version || 'Unknown'],
        ['System agent', data.software.agent_version || 'Unknown'],
        ['Hostname', data.system.hostname],
        ['Platform', `${data.system.platform} (${data.system.architecture})`],
        ['Kernel', data.system.kernel],
        [
          'Processor',
          `${data.hardware.cpu.model || 'Unknown'} · ${data.hardware.cpu.cores} cores, ${data.hardware.cpu.threads} threads`,
        ],
        ['Memory', formatBytes(data.hardware.memory.total)],
        ['Up for', formatUptime(data.system.uptime)],
        ['Docker', data.software.docker_version || 'Not installed'],
      ]
    : []
  return (
    <Card
      title="About this system"
      isLoading={isLoading}
      error={error ? getErrorMessage(error) : null}
      onRetry={() => refetch()}
    >
      <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-[max-content_1fr]">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-sm text-muted-foreground">{k}</dt>
            <dd className="text-sm font-medium break-words">{v}</dd>
          </div>
        ))}
      </dl>
    </Card>
  )
}

/**
 * Settings. (Every control here used to be local state: "Save" said
 * "Settings saved successfully" and saved nothing, and About filled gaps
 * with made-up numbers.)
 */
export function Settings() {
  const [active, setActive] = useState<Section>('system')
  const navigate = useNavigate()
  const isAdmin = useIsAdmin()

  return (
    <div className="space-y-6">
      <PageHeader title="Settings" description="This system and your account" />
      <div className="flex flex-col gap-6 md:flex-row">
        <div className="w-full shrink-0 space-y-4 md:w-64">
          <Card>
            <nav className="space-y-1" aria-label="Settings sections">
              {sections.map((s) => (
                <button
                  key={s.id}
                  onClick={() => setActive(s.id)}
                  aria-current={active === s.id ? 'page' : undefined}
                  className={cn(
                    'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors',
                    active === s.id ? 'bg-primary text-primary-foreground' : 'hover:bg-muted/50',
                  )}
                >
                  <s.icon className="h-4 w-4" aria-hidden />
                  {s.label}
                </button>
              ))}
            </nav>
          </Card>
          <Card title="More settings">
            <nav className="space-y-1">
              {otherPages
                .filter((p) => isAdmin || !p.adminOnly)
                .map((p) => (
                  <button
                    key={p.path}
                    onClick={() => navigate(p.path)}
                    className="flex w-full items-center justify-between rounded-lg p-2 text-left text-sm hover:bg-muted/50"
                  >
                    <span className="flex items-center gap-2">
                      <p.icon className="h-4 w-4 text-muted-foreground" aria-hidden />
                      {p.label}
                    </span>
                    <ChevronRightIcon className="h-4 w-4 text-muted-foreground" aria-hidden />
                  </button>
                ))}
            </nav>
          </Card>
        </div>
        <div className="flex-1">
          {active === 'system' && <SystemSection />}
          {active === 'account' && <AccountSection />}
          {active === 'about' && <AboutSection />}
        </div>
      </div>
    </div>
  )
}
