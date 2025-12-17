/**
 * Sync Conflicts Page
 * View and resolve file synchronization conflicts
 */

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  Check,
  Copy,
  FileText,
  Clock,
  HardDrive,
  Cloud,
  RefreshCw,
  ChevronRight,
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
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { toast } from 'sonner';

import {
  listConflicts,
  resolveConflict,
  syncKeys,
  SyncConflict,
  ConflictResolution,
  formatBytes,
  formatRelativeTime,
} from '@/api/sync';

export default function SyncConflicts() {
  const queryClient = useQueryClient();
  const [selectedConflict, setSelectedConflict] = useState<SyncConflict | null>(null);
  const [resolveDialogOpen, setResolveDialogOpen] = useState(false);

  // Fetch conflicts
  const {
    data: conflicts,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: syncKeys.conflicts(),
    queryFn: () => listConflicts(),
    refetchInterval: 30000, // Refresh every 30 seconds
  });

  // Resolve conflict mutation
  const resolveMutation = useMutation({
    mutationFn: ({
      conflictId,
      resolution,
    }: {
      conflictId: string;
      resolution: ConflictResolution;
    }) => resolveConflict(conflictId, resolution),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: syncKeys.conflicts() });
      setResolveDialogOpen(false);
      setSelectedConflict(null);
      toast.success('Conflict resolved successfully');
    },
    onError: (error: Error) => {
      toast.error('Failed to resolve conflict', {
        description: error.message,
      });
    },
  });

  const handleResolve = (resolution: ConflictResolution) => {
    if (!selectedConflict) return;
    resolveMutation.mutate({
      conflictId: selectedConflict.id,
      resolution,
    });
  };

  const openResolveDialog = (conflict: SyncConflict) => {
    setSelectedConflict(conflict);
    setResolveDialogOpen(true);
  };

  const unresolvedConflicts = conflicts?.filter((c) => !c.resolved) || [];
  const resolvedConflicts = conflicts?.filter((c) => c.resolved) || [];

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Sync Conflicts</h1>
          <p className="text-muted-foreground">
            Review and resolve file synchronization conflicts
          </p>
        </div>
        <Button variant="outline" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Stats Card */}
      <Card>
        <CardContent className="pt-6">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-yellow-500/10">
                <AlertTriangle className="h-5 w-5 text-yellow-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">{unresolvedConflicts.length}</p>
                <p className="text-sm text-muted-foreground">Unresolved</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-green-500/10">
                <Check className="h-5 w-5 text-green-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">{resolvedConflicts.length}</p>
                <p className="text-sm text-muted-foreground">Resolved</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-blue-500/10">
                <FileText className="h-5 w-5 text-blue-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">{conflicts?.length || 0}</p>
                <p className="text-sm text-muted-foreground">Total</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Unresolved Conflicts */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-yellow-500" />
            Unresolved Conflicts
          </CardTitle>
          <CardDescription>
            These files have conflicting changes that need your attention
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[...Array(3)].map((_, i) => (
                <Skeleton key={i} className="h-16 w-full" />
              ))}
            </div>
          ) : unresolvedConflicts.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="p-3 rounded-full bg-green-500/10 mb-4">
                <Check className="h-8 w-8 text-green-500" />
              </div>
              <h3 className="font-semibold text-lg mb-1">All Clear!</h3>
              <p className="text-muted-foreground">
                You have no file conflicts to resolve
              </p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>File</TableHead>
                  <TableHead>Local Version</TableHead>
                  <TableHead>Remote Version</TableHead>
                  <TableHead>Detected</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {unresolvedConflicts.map((conflict) => (
                  <TableRow key={conflict.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <div>
                          <p className="font-medium">
                            {conflict.path.split('/').pop()}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {conflict.path}
                          </p>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <HardDrive className="h-4 w-4 text-blue-500" />
                        <span>{formatBytes(conflict.local_version.size)}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Cloud className="h-4 w-4 text-green-500" />
                        <span>{formatBytes(conflict.remote_version.size)}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className="text-muted-foreground">
                        {formatRelativeTime(conflict.detected_at)}
                      </span>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        onClick={() => openResolveDialog(conflict)}
                      >
                        Resolve
                        <ChevronRight className="h-4 w-4 ml-1" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Resolved Conflicts (History) */}
      {resolvedConflicts.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              Resolution History
            </CardTitle>
            <CardDescription>
              Previously resolved conflicts
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>File</TableHead>
                  <TableHead>Resolution</TableHead>
                  <TableHead>Resolved</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {resolvedConflicts.slice(0, 10).map((conflict) => (
                  <TableRow key={conflict.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <FileText className="h-4 w-4 text-muted-foreground" />
                        <span>{conflict.path.split('/').pop()}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">
                        {conflict.resolution === 'keep_local'
                          ? 'Kept Local'
                          : conflict.resolution === 'keep_remote'
                          ? 'Kept Remote'
                          : 'Kept Both'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {formatRelativeTime(conflict.resolved_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* Resolve Dialog */}
      <Dialog open={resolveDialogOpen} onOpenChange={setResolveDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Resolve Conflict</DialogTitle>
            <DialogDescription>
              Choose how to resolve this file conflict
            </DialogDescription>
          </DialogHeader>

          {selectedConflict && (
            <div className="space-y-4">
              {/* File info */}
              <div className="p-3 bg-muted rounded-lg">
                <p className="font-medium">
                  {selectedConflict.path.split('/').pop()}
                </p>
                <p className="text-xs text-muted-foreground">
                  {selectedConflict.path}
                </p>
              </div>

              {/* Version comparison */}
              <div className="grid grid-cols-2 gap-4">
                <div className="p-3 border rounded-lg">
                  <div className="flex items-center gap-2 mb-2">
                    <HardDrive className="h-4 w-4 text-blue-500" />
                    <span className="font-medium">Local Version</span>
                  </div>
                  <p className="text-sm">
                    {formatBytes(selectedConflict.local_version.size)}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {formatRelativeTime(selectedConflict.local_version.modified)}
                  </p>
                </div>
                <div className="p-3 border rounded-lg">
                  <div className="flex items-center gap-2 mb-2">
                    <Cloud className="h-4 w-4 text-green-500" />
                    <span className="font-medium">Remote Version</span>
                  </div>
                  <p className="text-sm">
                    {formatBytes(selectedConflict.remote_version.size)}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {formatRelativeTime(selectedConflict.remote_version.modified)}
                  </p>
                </div>
              </div>
            </div>
          )}

          <DialogFooter className="flex-col sm:flex-row gap-2">
            <Button
              variant="outline"
              onClick={() => handleResolve('keep_local')}
              disabled={resolveMutation.isPending}
              className="flex-1"
            >
              <HardDrive className="h-4 w-4 mr-2" />
              Keep Local
            </Button>
            <Button
              variant="outline"
              onClick={() => handleResolve('keep_remote')}
              disabled={resolveMutation.isPending}
              className="flex-1"
            >
              <Cloud className="h-4 w-4 mr-2" />
              Keep Remote
            </Button>
            <Button
              onClick={() => handleResolve('keep_both')}
              disabled={resolveMutation.isPending}
              className="flex-1"
            >
              <Copy className="h-4 w-4 mr-2" />
              Keep Both
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

