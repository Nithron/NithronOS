import { useState } from 'react'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { PageHeader } from '@/components/ui/page-header'
import { Card } from '@/components/ui/card-enhanced'
import { Button } from '@/components/ui/button'
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
import { getErrorMessage } from '@/lib/nos-client'
import { groupNameProblem, useGroupMutations, useGroups, type Group } from '@/api/groups'
import { useUserAccounts } from '@/api/users'

/** GroupDialog creates a group (group === null) or edits one. */
function GroupDialog({
  open,
  group,
  onClose,
}: {
  open: boolean
  group: Group | null
  onClose: () => void
}) {
  const { create, update } = useGroupMutations()
  const { data: users = [] } = useUserAccounts()
  const [name, setName] = useState(group?.name ?? '')
  const [description, setDescription] = useState(group?.description ?? '')
  const [members, setMembers] = useState<string[]>(group?.members ?? [])
  const editing = group !== null
  const nameProblem = editing ? null : name ? groupNameProblem(name) : null
  const busy = create.isPending || update.isPending

  const toggle = (username: string, on: boolean) =>
    setMembers((m) =>
      on ? [...m.filter((x) => x !== username), username] : m.filter((x) => x !== username),
    )

  const save = () => {
    const done = {
      onSuccess: () => {
        toast.success(editing ? `Group ${group!.name} saved` : `Group ${name} created`)
        onClose()
      },
      onError: (e: unknown) => toast.error(getErrorMessage(e)),
    }
    if (editing) update.mutate({ name: group!.name, data: { description, members } }, done)
    else create.mutate({ name, description, members }, done)
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? `Edit ${group!.name}` : 'New group'}</DialogTitle>
          <DialogDescription>
            Give a share's access to a group, and everyone in it gets that access.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {!editing && (
            <div className="space-y-2">
              <Label htmlFor="group-name">Name</Label>
              <Input
                id="group-name"
                value={name}
                onChange={(e) => setName(e.target.value.trim())}
                placeholder="family"
              />
              {nameProblem && <p className="text-sm text-destructive">{nameProblem}</p>}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="group-description">Description</Label>
            <Input
              id="group-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <fieldset className="space-y-2">
            <legend className="text-sm font-medium">Members</legend>
            {users.length === 0 && <p className="text-sm text-muted-foreground">No users yet.</p>}
            <div className="max-h-56 space-y-1 overflow-y-auto">
              {users.map((u) => (
                <label
                  key={u.id}
                  className="flex items-center gap-2 rounded p-1 text-sm hover:bg-muted/50"
                >
                  <input
                    type="checkbox"
                    checked={members.includes(u.username)}
                    onChange={(e) => toggle(u.username, e.target.checked)}
                  />
                  {u.username}
                </label>
              ))}
            </div>
          </fieldset>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || (!editing && (!name || nameProblem !== null))}>
            {busy ? 'Saving…' : editing ? 'Save' : 'Create group'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/**
 * Groups: named sets of users that shares can be given to (the Unix groups
 * nos-<name> that Samba, NFS and the folder permissions use). "Everyone" is
 * built in.
 */
export function Groups() {
  const { data: groups, isLoading, error, refetch } = useGroups()
  const { remove } = useGroupMutations()
  const [editing, setEditing] = useState<Group | null>(null)
  const [creating, setCreating] = useState(false)

  const del = (g: Group) =>
    remove.mutate(g.name, {
      onSuccess: () => toast.success(`Group ${g.name} deleted`),
      // The backend refuses a group shares still use, and says which.
      onError: (e) => toast.error(getErrorMessage(e)),
    })

  return (
    <div className="space-y-6">
      <PageHeader
        title="Groups"
        description="Sets of users that shares can be given to"
        actions={
          <Button onClick={() => setCreating(true)}>
            <Plus className="mr-2 h-4 w-4" aria-hidden />
            New group
          </Button>
        }
      />
      <Card
        isLoading={isLoading}
        error={error ? getErrorMessage(error) : null}
        onRetry={() => refetch()}
      >
        {groups && groups.length === 0 && (
          <p className="text-sm text-muted-foreground">
            No groups yet. Every user is already in “Everyone”; make a group to give a share to only
            some of them.
          </p>
        )}
        {groups && groups.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Members</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {groups.map((g) => (
                <TableRow key={g.name}>
                  <TableCell>
                    <div className="font-medium">{g.name}</div>
                    {g.description && (
                      <div className="text-xs text-muted-foreground">{g.description}</div>
                    )}
                  </TableCell>
                  <TableCell className="text-sm">
                    {g.members.length ? g.members.join(', ') : 'Nobody'}
                  </TableCell>
                  <TableCell>
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setEditing(g)} title="Edit">
                        <Pencil className="h-4 w-4" aria-hidden />
                        <span className="sr-only">Edit {g.name}</span>
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => del(g)}
                        disabled={remove.isPending}
                        title="Delete"
                      >
                        <Trash2 className="h-4 w-4 text-destructive" aria-hidden />
                        <span className="sr-only">Delete {g.name}</span>
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Card>
      {creating && <GroupDialog open group={null} onClose={() => setCreating(false)} />}
      {editing && (
        <GroupDialog key={editing.name} open group={editing} onClose={() => setEditing(null)} />
      )}
    </div>
  )
}
