/**
 * Sync Sharing Page
 * Manage shared folders and invitations
 */

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  FolderOpen,
  Users,
  UserPlus,
  Mail,
  Trash2,
  RefreshCw,
  Plus,
  Crown,
  Edit,
  Eye,
  Check,
  X,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { toast } from 'sonner';

import {
  listSharedFolders,
  listPendingInvites,
  createSharedFolder,
  deleteSharedFolder,
  removeFolderMember,
  acceptInvite,
  declineInvite,
  listSyncShares,
  syncKeys,
  SharedFolder,
  SharePermission,
  formatRelativeTime,
} from '@/api/sync';

export default function SyncSharing() {
  const queryClient = useQueryClient();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedFolder, setSelectedFolder] = useState<SharedFolder | null>(null);
  const [manageMembersOpen, setManageMembersOpen] = useState(false);
  const [, setAddMemberOpen] = useState(false);

  // Form state
  const [newFolderName, setNewFolderName] = useState('');
  const [newFolderPath, setNewFolderPath] = useState('/');
  const [selectedShareId, setSelectedShareId] = useState('');
  const [, setNewMemberEmail] = useState('');
  const [, setNewMemberPermission] = useState<SharePermission>('read');

  // Fetch shared folders
  const {
    data: foldersData,
    isLoading: loadingFolders,
    refetch: refetchFolders,
  } = useQuery({
    queryKey: syncKeys.sharedFolders(),
    queryFn: listSharedFolders,
  });

  // Fetch pending invites
  const {
    data: invitesData,
    isLoading: loadingInvites,
    refetch: refetchInvites,
  } = useQuery({
    queryKey: syncKeys.invites(),
    queryFn: listPendingInvites,
  });

  // Fetch shares for the create dialog
  const { data: sharesData } = useQuery({
    queryKey: syncKeys.shares(),
    queryFn: listSyncShares,
  });

  // Create folder mutation
  const createMutation = useMutation({
    mutationFn: () =>
      createSharedFolder(selectedShareId, newFolderPath, newFolderName, 'Current User'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.sharedFolders() });
      setCreateDialogOpen(false);
      resetForm();
      toast.success('Shared folder created');
    },
    onError: (error: Error) => {
      toast.error('Failed to create folder', { description: error.message });
    },
  });

  // Delete folder mutation
  const deleteMutation = useMutation({
    mutationFn: (folderId: string) => deleteSharedFolder(folderId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.sharedFolders() });
      toast.success('Shared folder deleted');
    },
    onError: (error: Error) => {
      toast.error('Failed to delete folder', { description: error.message });
    },
  });

  // Accept invite mutation
  const acceptMutation = useMutation({
    mutationFn: (inviteId: string) => acceptInvite(inviteId, 'Current User'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.invites() });
      queryClient.invalidateQueries({ queryKey: syncKeys.sharedFolders() });
      toast.success('Invitation accepted');
    },
    onError: (error: Error) => {
      toast.error('Failed to accept invitation', { description: error.message });
    },
  });

  // Decline invite mutation
  const declineMutation = useMutation({
    mutationFn: (inviteId: string) => declineInvite(inviteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.invites() });
      toast.success('Invitation declined');
    },
    onError: (error: Error) => {
      toast.error('Failed to decline invitation', { description: error.message });
    },
  });

  // Remove member mutation
  const removeMemberMutation = useMutation({
    mutationFn: ({ folderId, userId }: { folderId: string; userId: string }) =>
      removeFolderMember(folderId, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.sharedFolders() });
      toast.success('Member removed');
    },
    onError: (error: Error) => {
      toast.error('Failed to remove member', { description: error.message });
    },
  });

  const resetForm = () => {
    setNewFolderName('');
    setNewFolderPath('/');
    setSelectedShareId('');
    setNewMemberEmail('');
    setNewMemberPermission('read');
  };

  const getPermissionIcon = (permission: SharePermission) => {
    switch (permission) {
      case 'admin':
        return <Crown className="h-4 w-4 text-yellow-500" />;
      case 'write':
        return <Edit className="h-4 w-4 text-blue-500" />;
      case 'read':
        return <Eye className="h-4 w-4 text-gray-500" />;
    }
  };

  const getPermissionBadge = (permission: SharePermission) => {
    const colors = {
      admin: 'bg-yellow-500/10 text-yellow-600',
      write: 'bg-blue-500/10 text-blue-600',
      read: 'bg-gray-500/10 text-gray-600',
    };
    return (
      <Badge variant="secondary" className={colors[permission]}>
        {getPermissionIcon(permission)}
        <span className="ml-1 capitalize">{permission}</span>
      </Badge>
    );
  };

  const folders = foldersData?.folders || [];
  const invites = invitesData?.invites || [];

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Shared Folders</h1>
          <p className="text-muted-foreground">
            Collaborate on folders with other users
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => { refetchFolders(); refetchInvites(); }}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          <Button onClick={() => setCreateDialogOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            Share Folder
          </Button>
        </div>
      </div>

      {/* Pending Invites Banner */}
      {invites.length > 0 && (
        <Card className="border-blue-500/50 bg-blue-500/5">
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <Mail className="h-5 w-5 text-blue-500" />
              <div className="flex-1">
                <p className="font-medium">
                  You have {invites.length} pending invitation{invites.length > 1 ? 's' : ''}
                </p>
                <p className="text-sm text-muted-foreground">
                  Accept or decline to manage access to shared folders
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Tabs defaultValue="folders">
        <TabsList>
          <TabsTrigger value="folders">
            <FolderOpen className="h-4 w-4 mr-2" />
            My Folders ({folders.length})
          </TabsTrigger>
          <TabsTrigger value="invites">
            <Mail className="h-4 w-4 mr-2" />
            Invitations ({invites.length})
          </TabsTrigger>
        </TabsList>

        {/* Folders Tab */}
        <TabsContent value="folders" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Shared Folders</CardTitle>
              <CardDescription>
                Folders you own or have been shared with you
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loadingFolders ? (
                <div className="space-y-3">
                  {[...Array(3)].map((_, i) => (
                    <Skeleton key={i} className="h-16 w-full" />
                  ))}
                </div>
              ) : folders.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <FolderOpen className="h-12 w-12 text-muted-foreground mb-4" />
                  <h3 className="font-semibold text-lg mb-1">No Shared Folders</h3>
                  <p className="text-muted-foreground mb-4">
                    Create a shared folder to collaborate with others
                  </p>
                  <Button onClick={() => setCreateDialogOpen(true)}>
                    <Plus className="h-4 w-4 mr-2" />
                    Share Your First Folder
                  </Button>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Folder</TableHead>
                      <TableHead>Owner</TableHead>
                      <TableHead>Members</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {folders.map((folder) => (
                      <TableRow key={folder.id}>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <FolderOpen className="h-5 w-5 text-yellow-500" />
                            <div>
                              <p className="font-medium">{folder.name}</p>
                              <p className="text-xs text-muted-foreground">{folder.path}</p>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>{folder.owner_name}</TableCell>
                        <TableCell>
                          <div className="flex items-center gap-1">
                            <Users className="h-4 w-4 text-muted-foreground" />
                            <span>{folder.members.length}</span>
                          </div>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatRelativeTime(folder.created_at)}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-2">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setSelectedFolder(folder);
                                setManageMembersOpen(true);
                              }}
                            >
                              <Users className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => deleteMutation.mutate(folder.id)}
                            >
                              <Trash2 className="h-4 w-4 text-red-500" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Invitations Tab */}
        <TabsContent value="invites" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle>Pending Invitations</CardTitle>
              <CardDescription>
                Invitations to join shared folders
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loadingInvites ? (
                <div className="space-y-3">
                  {[...Array(2)].map((_, i) => (
                    <Skeleton key={i} className="h-16 w-full" />
                  ))}
                </div>
              ) : invites.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <Mail className="h-12 w-12 text-muted-foreground mb-4" />
                  <h3 className="font-semibold text-lg mb-1">No Pending Invitations</h3>
                  <p className="text-muted-foreground">
                    You'll see invitations here when someone shares a folder with you
                  </p>
                </div>
              ) : (
                <div className="space-y-4">
                  {invites.map((invite) => (
                    <div
                      key={invite.id}
                      className="flex items-center justify-between p-4 border rounded-lg"
                    >
                      <div className="flex items-center gap-4">
                        <div className="p-2 rounded-lg bg-blue-500/10">
                          <FolderOpen className="h-6 w-6 text-blue-500" />
                        </div>
                        <div>
                          <p className="font-medium">{invite.folder_name}</p>
                          <p className="text-sm text-muted-foreground">
                            Invited by {invite.inviter_name}
                          </p>
                          {invite.message && (
                            <p className="text-sm italic mt-1">"{invite.message}"</p>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center gap-4">
                        {getPermissionBadge(invite.permission)}
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            onClick={() => acceptMutation.mutate(invite.id)}
                            disabled={acceptMutation.isPending}
                          >
                            <Check className="h-4 w-4 mr-1" />
                            Accept
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => declineMutation.mutate(invite.id)}
                            disabled={declineMutation.isPending}
                          >
                            <X className="h-4 w-4 mr-1" />
                            Decline
                          </Button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Create Folder Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Share a Folder</DialogTitle>
            <DialogDescription>
              Create a shared folder that others can access
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Sync Share</Label>
              <Select value={selectedShareId} onValueChange={setSelectedShareId}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a share" />
                </SelectTrigger>
                <SelectContent>
                  {sharesData?.shares.map((share) => (
                    <SelectItem key={share.share_id} value={share.share_id}>
                      {share.share_name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Folder Path</Label>
              <Input
                value={newFolderPath}
                onChange={(e) => setNewFolderPath(e.target.value)}
                placeholder="/path/to/folder"
              />
            </div>
            <div className="space-y-2">
              <Label>Display Name</Label>
              <Input
                value={newFolderName}
                onChange={(e) => setNewFolderName(e.target.value)}
                placeholder="Shared Documents"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => createMutation.mutate()}
              disabled={!selectedShareId || !newFolderPath || !newFolderName || createMutation.isPending}
            >
              Create Shared Folder
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Manage Members Dialog */}
      <Dialog open={manageMembersOpen} onOpenChange={setManageMembersOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Manage Members</DialogTitle>
            <DialogDescription>
              {selectedFolder?.name} - {selectedFolder?.members.length} members
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            {selectedFolder?.members.map((member) => (
              <div
                key={member.user_id}
                className="flex items-center justify-between p-3 border rounded-lg"
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                    <span className="text-sm font-medium">
                      {member.username.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div>
                    <p className="font-medium">{member.username}</p>
                    <p className="text-xs text-muted-foreground">
                      Added {formatRelativeTime(member.added_at)}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {getPermissionBadge(member.permission)}
                  {member.user_id !== selectedFolder?.owner_id && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        removeMemberMutation.mutate({
                          folderId: selectedFolder.id,
                          userId: member.user_id,
                        })
                      }
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setManageMembersOpen(false)}>
              Close
            </Button>
            <Button onClick={() => setAddMemberOpen(true)}>
              <UserPlus className="h-4 w-4 mr-2" />
              Add Member
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

