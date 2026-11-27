import { useState } from 'react'
import { AddUserIcon, DeleteIcon, KeyIcon, SecurityOffIcon } from '@/components/icons'
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
import { useAuth } from '@/lib/auth'
import { getErrorMessage } from '@/lib/nos-client'
import {
  USERNAME_PATTERN,
  passwordProblem,
  useUserAccounts,
  useUserMutations,
  type Role,
  type UserAccount,
} from '@/api/users'

const roleLabel: Record<Role, string> = { admin: 'Administrator', user: 'User' }

function roleOf(u: UserAccount): Role {
  return u.roles.includes('admin') ? 'admin' : 'user'
}

function when(iso?: string): string {
  return iso ? new Date(iso).toLocaleString() : 'Never'
}

/** PasswordFields is a new password typed twice. */
function PasswordFields({
  value,
  confirm,
  onChange,
  onConfirm,
}: {
  value: string
  confirm: string
  onChange: (v: string) => void
  onConfirm: (v: string) => void
}) {
  const problem = value ? passwordProblem(value) : null
  const mismatch = confirm !== '' && confirm !== value
  return (
    <>
      <div className="space-y-2">
        <Label htmlFor="pw">Password</Label>
        <Input
          id="pw"
          type="password"
          autoComplete="new-password"
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
        <p className={problem ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}>
          {problem ?? 'Also the password for the file shares (SMB).'}
        </p>
      </div>
      <div className="space-y-2">
        <Label htmlFor="pw2">Repeat password</Label>
        <Input
          id="pw2"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => onConfirm(e.target.value)}
        />
        {mismatch && <p className="text-sm text-destructive">The passwords don't match</p>}
      </div>
    </>
  )
}

function passwordReady(p: string, confirm: string): boolean {
  return passwordProblem(p) === null && p === confirm
}

function AddUserDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const { create } = useUserMutations()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [role, setRole] = useState<Role>('user')
  const nameOK = USERNAME_PATTERN.test(username)

  const close = () => {
    setUsername('')
    setPassword('')
    setConfirm('')
    setRole('user')
    onOpenChange(false)
  }
  const submit = () =>
    create.mutate(
      { username, password, roles: [role] },
      {
        onSuccess: () => {
          toast.success(`User ${username} created`)
          close()
        },
        onError: (e) => toast.error(getErrorMessage(e)),
      },
    )

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add user</DialogTitle>
          <DialogDescription>
            The user signs in to nOS and to the file shares with this name and password.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              autoComplete="off"
              value={username}
              onChange={(e) => setUsername(e.target.value.trim())}
            />
            <p
              className={
                username && !nameOK ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'
              }
            >
              3-32 lowercase letters, digits, '-' or '_', starting with a letter
            </p>
          </div>
          <PasswordFields
            value={password}
            confirm={confirm}
            onChange={setPassword}
            onConfirm={setConfirm}
          />
          <div className="space-y-2">
            <Label htmlFor="role">Role</Label>
            <select
              id="role"
              className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
              value={role}
              onChange={(e) => setRole(e.target.value as Role)}
            >
              <option value="user">User: the shares, apps and sync they are given</option>
              <option value="admin">Administrator: manages the whole system</option>
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={close}>
            Cancel
          </Button>
          <Button
            onClick={submit}
            disabled={!nameOK || !passwordReady(password, confirm) || create.isPending}
          >
            {create.isPending ? 'Creating…' : 'Create user'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ResetPasswordDialog({ user, onClose }: { user: UserAccount | null; onClose: () => void }) {
  const { resetPassword } = useUserMutations()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const close = () => {
    setPassword('')
    setConfirm('')
    onClose()
  }
  const submit = () => {
    if (!user) return
    resetPassword.mutate(
      { id: user.id, password },
      {
        onSuccess: () => {
          toast.success(`New password set for ${user.username}; they were signed out`)
          close()
        },
        onError: (e) => toast.error(getErrorMessage(e)),
      },
    )
  }
  return (
    <Dialog open={user !== null} onOpenChange={(o) => !o && close()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Set a new password for {user?.username}</DialogTitle>
          <DialogDescription>
            Their devices are signed out, and a lockout from failed sign-ins ends.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <PasswordFields
            value={password}
            confirm={confirm}
            onChange={setPassword}
            onConfirm={setConfirm}
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={close}>
            Cancel
          </Button>
          <Button
            onClick={submit}
            disabled={!passwordReady(password, confirm) || resetPassword.isPending}
          >
            {resetPassword.isPending ? 'Saving…' : 'Set password'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function DeleteUserDialog({ user, onClose }: { user: UserAccount | null; onClose: () => void }) {
  const { remove } = useUserMutations()
  const submit = () => {
    if (!user) return
    remove.mutate(user.id, {
      onSuccess: () => {
        toast.success(`User ${user.username} deleted`)
        onClose()
      },
      onError: (e) => toast.error(getErrorMessage(e)),
    })
  }
  return (
    <Dialog open={user !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete {user?.username}?</DialogTitle>
          <DialogDescription>
            They are signed out, lose their file-sharing access, and are taken out of every group
            and share. Files they made in shares stay.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={submit} disabled={remove.isPending}>
            {remove.isPending ? 'Deleting…' : 'Delete user'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function UserRow({
  user,
  isSelf,
  onReset,
  onDelete,
}: {
  user: UserAccount
  isSelf: boolean
  onReset: () => void
  onDelete: () => void
}) {
  const { setRoles, disableTwoFactor } = useUserMutations()
  const role = roleOf(user)
  const changeRole = (next: Role) =>
    setRoles.mutate(
      { id: user.id, roles: [next] },
      {
        onSuccess: () => toast.success(`${user.username} is now ${roleLabel[next].toLowerCase()}`),
        onError: (e) => toast.error(getErrorMessage(e)),
      },
    )
  const turnOff2FA = () =>
    disableTwoFactor.mutate(user.id, {
      onSuccess: () => toast.success(`Two-factor sign-in is off for ${user.username}`),
      onError: (e) => toast.error(getErrorMessage(e)),
    })

  return (
    <TableRow>
      <TableCell className="font-medium">
        {user.username}
        {isSelf && <span className="ml-2 text-xs text-muted-foreground">(you)</span>}
      </TableCell>
      <TableCell>
        <select
          aria-label={`Role of ${user.username}`}
          className="h-9 rounded-md border border-input bg-background px-2 text-sm"
          value={role}
          disabled={setRoles.isPending}
          onChange={(e) => changeRole(e.target.value as Role)}
        >
          <option value="user">{roleLabel.user}</option>
          <option value="admin">{roleLabel.admin}</option>
        </select>
      </TableCell>
      <TableCell>
        {user.two_factor_enabled ? (
          <Badge>On</Badge>
        ) : (
          <span className="text-muted-foreground">Off</span>
        )}
        {user.locked_until && (
          <Badge variant="destructive" className="ml-2">
            Locked
          </Badge>
        )}
      </TableCell>
      <TableCell className="text-muted-foreground">{when(user.last_login_at)}</TableCell>
      <TableCell>
        <div className="flex justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={onReset} title="Set a new password">
            <KeyIcon className="h-4 w-4" aria-hidden />
            <span className="sr-only">Set a new password for {user.username}</span>
          </Button>
          {user.two_factor_enabled && !isSelf && (
            <Button
              variant="ghost"
              size="sm"
              onClick={turnOff2FA}
              disabled={disableTwoFactor.isPending}
              title="Turn off two-factor sign-in"
            >
              <SecurityOffIcon className="h-4 w-4" aria-hidden />
              <span className="sr-only">Turn off two-factor sign-in for {user.username}</span>
            </Button>
          )}
          {!isSelf && (
            <Button variant="ghost" size="sm" onClick={onDelete} title="Delete user">
              <DeleteIcon className="h-4 w-4 text-destructive" aria-hidden />
              <span className="sr-only">Delete {user.username}</span>
            </Button>
          )}
        </div>
      </TableCell>
    </TableRow>
  )
}

/**
 * Users: who can sign in to nOS and the file shares. (This page used to show
 * made-up emails and "coming soon" for every action, and "deleted" users
 * without deleting them.)
 */
export function Users() {
  const { session } = useAuth()
  const { data: users, isLoading, error, refetch } = useUserAccounts()
  const [adding, setAdding] = useState(false)
  const [resetting, setResetting] = useState<UserAccount | null>(null)
  const [deleting, setDeleting] = useState<UserAccount | null>(null)

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        description="People who sign in to nOS and to the file shares"
        actions={
          <Button onClick={() => setAdding(true)}>
            <AddUserIcon className="mr-2 h-4 w-4" aria-hidden />
            Add user
          </Button>
        }
      />
      <Card
        isLoading={isLoading}
        error={error ? getErrorMessage(error) : null}
        onRetry={() => refetch()}
      >
        {users && users.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Two-factor</TableHead>
                <TableHead>Last sign-in</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <UserRow
                  key={u.id}
                  user={u}
                  isSelf={u.id === session?.user.id}
                  onReset={() => setResetting(u)}
                  onDelete={() => setDeleting(u)}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </Card>
      <AddUserDialog open={adding} onOpenChange={setAdding} />
      <ResetPasswordDialog user={resetting} onClose={() => setResetting(null)} />
      <DeleteUserDialog user={deleting} onClose={() => setDeleting(null)} />
    </div>
  )
}
