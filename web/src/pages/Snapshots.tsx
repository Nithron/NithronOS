import { useState } from 'react';
import { confirmAction } from '@/components/ui/confirm';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { AddIcon, DeleteIcon, PoolIcon, RefreshIcon, WarningIcon } from '@/components/icons'
import { backupApi } from '@/api/backup';
import type { Snapshot } from '@/api/backup.types';
import { formatBytes } from '@/lib/utils';
import { toast } from '@/components/ui/toast';

export default function Snapshots() {
  const queryClient = useQueryClient();
  const [selectedSubvolume, setSelectedSubvolume] = useState<string>('');
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showRestoreDialog, setShowRestoreDialog] = useState(false);
  const [selectedSnapshot, setSelectedSnapshot] = useState<Snapshot | null>(null);
  
  // Fetch snapshots
  const { data: snapshotsData, isLoading } = useQuery({
    queryKey: ['snapshots', selectedSubvolume],
    queryFn: () => backupApi.snapshots.list(selectedSubvolume),
  });
  
  // Fetch stats
  const { data: stats } = useQuery({
    queryKey: ['snapshot-stats'],
    queryFn: () => backupApi.snapshots.stats(),
  });
  
  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => backupApi.snapshots.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['snapshots'] });
      queryClient.invalidateQueries({ queryKey: ['snapshot-stats'] });
      toast.success('Snapshot deleted successfully');
    },
    onError: () => {
      toast.error('Failed to delete snapshot');
    },
  });
  
  const snapshots = snapshotsData?.snapshots || [];
  const subvolumes = stats ? Object.keys(stats.by_subvolume) : [];
  
  const handleDelete = async (snapshot: Snapshot) => {
    if (await confirmAction({ title: 'Delete this snapshot?', description: <span className="font-mono">{snapshot.path}</span>, action: 'Delete snapshot' })) {
      deleteMutation.mutate(snapshot.id);
    }
  };
  
  const handleRestore = (snapshot: Snapshot) => {
    setSelectedSnapshot(snapshot);
    setShowRestoreDialog(true);
  };
  
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="screen-title">Snapshots</h1>
          <p className="text-muted-foreground">
            Manage system snapshots and restore points
          </p>
        </div>
        <button
          onClick={() => setShowCreateDialog(true)}
          className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
        >
          <AddIcon className="w-4 h-4 mr-2" />
          Create snapshot
        </button>
      </div>
      
      {/* Stats Cards */}
      {stats && (
        <div className="grid gap-px border border-hairline bg-hairline md:grid-cols-3 [&>*]:bg-page [&>*]:p-4">
          <div>
            <p className="font-mono text-[0.8125rem] text-ink-2">Snapshots</p>
            <p className="pixel-display pixel-24 mt-2">{stats.total_count}</p>
          </div>
          <div>
            <p className="font-mono text-[0.8125rem] text-ink-2">Total size</p>
            <p className="pixel-display pixel-24 mt-2">{formatBytes(stats.total_size_bytes)}</p>
          </div>
          <div>
            <p className="font-mono text-[0.8125rem] text-ink-2">Oldest</p>
            <p className="mt-2 text-2xl">
              {stats.oldest_snapshot ? formatDistanceToNow(new Date(stats.oldest_snapshot), { addSuffix: true }) : 'None yet'}
            </p>
          </div>
        </div>
      )}
      
      {/* Subvolume Filter */}
      {subvolumes.length > 0 && (
        <div className="flex items-center space-x-4">
          <label className="text-sm font-medium">Filter by subvolume:</label>
          <select
            value={selectedSubvolume}
            onChange={(e) => setSelectedSubvolume(e.target.value)}
            className="field h-8 w-auto"
          >
            <option value="">All subvolumes</option>
            {subvolumes.map((subvol) => (
              <option key={subvol} value={subvol}>{subvol}</option>
            ))}
          </select>
        </div>
      )}
      
      {/* Snapshots List */}
      <div className="border border-hairline">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading snapshots...
          </div>
        ) : snapshots.length === 0 ? (
          <div className="p-8 text-center">
            <PoolIcon className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground mb-4">No snapshots found</p>
            <button
              onClick={() => setShowCreateDialog(true)}
              className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
            >
              <AddIcon className="w-4 h-4 mr-2" />
              Create first snapshot
            </button>
          </div>
        ) : (
          <div className="divide-y">
            {snapshots.map((snapshot) => (
              <div key={snapshot.id} className="p-4 hover:bg-ink/[.09] transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex-1">
                    <div className="flex items-center space-x-3">
                      <PoolIcon className="w-5 h-5 text-primary" />
                      <div>
                        <p className="font-medium">{snapshot.path}</p>
                        <div className="flex items-center space-x-4 mt-1 text-sm text-muted-foreground">
                          <span>Subvolume: {snapshot.subvolume}</span>
                          <span>•</span>
                          <span>
                            Created {formatDistanceToNow(new Date(snapshot.created_at), { addSuffix: true })}
                          </span>
                          {(snapshot.size_bytes ?? 0) > 0 && (
                            <>
                              <span>•</span>
                              <span>{formatBytes(snapshot.size_bytes ?? 0)}</span>
                            </>
                          )}
                          {snapshot.schedule_id && (
                            <>
                              <span>•</span>
                              <span className="text-status-ok-text">Scheduled</span>
                            </>
                          )}
                        </div>
                        {snapshot.tags && snapshot.tags.length > 0 && (
                          <div className="flex items-center space-x-2 mt-2">
                            {snapshot.tags.map((tag: any) => (
                              <span
                                key={tag}
                                className="rounded-sm border border-rule px-2 py-0.5 text-xs text-ink"
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => handleRestore(snapshot)}
                      className="btn btn-line min-h-8 px-3"
                      title="Restore from this snapshot"
                    >
                      <RefreshIcon className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(snapshot)}
                      className="btn btn-line min-h-8 px-3 hover:bg-ink/[.09]"
                      title="Delete snapshot"
                      disabled={deleteMutation.isPending}
                    >
                      <DeleteIcon className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      
      {/* Create Snapshot Dialog */}
      {showCreateDialog && (
        <CreateSnapshotDialog
          onClose={() => setShowCreateDialog(false)}
          onSuccess={() => {
            setShowCreateDialog(false);
            queryClient.invalidateQueries({ queryKey: ['snapshots'] });
            queryClient.invalidateQueries({ queryKey: ['snapshot-stats'] });
          }}
        />
      )}
      
      {/* Restore Dialog */}
      {showRestoreDialog && selectedSnapshot && (
        <RestoreDialog
          snapshot={selectedSnapshot}
          onClose={() => {
            setShowRestoreDialog(false);
            setSelectedSnapshot(null);
          }}
        />
      )}
    </div>
  );
}

// Create Snapshot Dialog Component
function CreateSnapshotDialog({ 
  onClose, 
  onSuccess 
}: { 
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [subvolumes, setSubvolumes] = useState<string[]>(['@']);
  const [tag, setTag] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  
  const handleCreate = async () => {
    if (subvolumes.length === 0) {
      toast.error('Select at least one subvolume');
      return;
    }
    
    setIsCreating(true);
    try {
      await backupApi.snapshots.create({ subvolumes, tag: tag || undefined });
      toast.success('Snapshot creation started');
      onSuccess();
    } catch {
      toast.error('Failed to create snapshot');
    } finally {
      setIsCreating(false);
    }
  };
  
  const toggleSubvolume = (subvol: string) => {
    setSubvolumes((prev) =>
      prev.includes(subvol)
        ? prev.filter((s) => s !== subvol)
        : [...prev, subvol]
    );
  };
  
  const availableSubvolumes = ['@', '@home', '@var', '@log'];
  
  return (
    <div role="dialog" aria-modal="true" className="fixed inset-0 z-50 flex items-center justify-center bg-page/70">
      <div className="border border-edge bg-page p-6 max-w-md w-full">
        <h2 className="text-lg font-[560] mb-4">Create snapshot</h2>
        
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">
              Select subvolumes
            </label>
            <div className="space-y-2">
              {availableSubvolumes.map((subvol) => (
                <label key={subvol} className="flex items-center">
                  <input
                    type="checkbox"
                    checked={subvolumes.includes(subvol)}
                    onChange={() => toggleSubvolume(subvol)}
                    className="mr-2"
                  />
                  <span>{subvol}</span>
                </label>
              ))}
            </div>
          </div>
          
          <div>
            <label htmlFor="tag" className="block text-sm font-medium mb-2">
              Tag (optional)
            </label>
            <input
              id="tag"
              type="text"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              placeholder="e.g., before-upgrade"
              className="field"
            />
          </div>
        </div>
        
        <div className="flex justify-end space-x-2 mt-6">
          <button
            onClick={onClose}
            className="btn"
            disabled={isCreating}
          >
            Cancel
          </button>
          <button
            onClick={handleCreate}
            className="btn btn-primary"
            disabled={isCreating}
          >
            {isCreating ? 'Creating...' : 'Create snapshot'}
          </button>
        </div>
      </div>
    </div>
  );
}

// Restore Dialog Component
function RestoreDialog({ 
  snapshot, 
  onClose 
}: { 
  snapshot: Snapshot;
  onClose: () => void;
}) {
  const [restoreType, setRestoreType] = useState<'full' | 'files'>('files');
  const [targetPath, setTargetPath] = useState('');
  const [isCreatingPlan, setIsCreatingPlan] = useState(false);
  const [plan, setPlan] = useState<any>(null);
  
  const handleCreatePlan = async () => {
    if (!targetPath) {
      toast.error('Enter a target path');
      return;
    }
    
    setIsCreatingPlan(true);
    try {
      const result = await backupApi.restore.createPlan({
        source_type: 'local',
        source_id: snapshot.id,
        restore_type: restoreType,
        target_path: targetPath,
      });
      setPlan(result);
    } catch {
      toast.error('Failed to create restore plan');
    } finally {
      setIsCreatingPlan(false);
    }
  };
  
  const handleApplyRestore = async () => {
    if (!plan) return;
    
    try {
      await backupApi.restore.apply({
        source_type: 'local',
        source_id: snapshot.id,
        restore_type: restoreType,
        target_path: targetPath,
      });
      toast.success('Restore started');
      onClose();
    } catch {
      toast.error('Failed to start restore');
    }
  };
  
  return (
    <div role="dialog" aria-modal="true" className="fixed inset-0 z-50 flex items-center justify-center bg-page/70">
      <div className="border border-edge bg-page p-6 max-w-2xl w-full max-h-[80vh] overflow-y-auto">
        <h2 className="text-lg font-[560] mb-4">Restore from snapshot</h2>
        
        <div className="border border-hairline p-4 mb-4">
          <p className="text-sm text-muted-foreground">Restoring from:</p>
          <p className="font-medium">{snapshot.path}</p>
          <p className="text-sm text-muted-foreground mt-1">
            Created {formatDistanceToNow(new Date(snapshot.created_at), { addSuffix: true })}
          </p>
        </div>
        
        {!plan ? (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">
                Restore type
              </label>
              <div className="space-y-2">
                <label className="flex items-center">
                  <input
                    type="radio"
                    value="files"
                    checked={restoreType === 'files'}
                    onChange={() => setRestoreType('files')}
                    className="mr-2"
                  />
                  <div>
                    <span>File-level restore</span>
                    <p className="text-xs text-muted-foreground">
                      Copy specific files without affecting the system
                    </p>
                  </div>
                </label>
                <label className="flex items-center">
                  <input
                    type="radio"
                    value="full"
                    checked={restoreType === 'full'}
                    onChange={() => setRestoreType('full')}
                    className="mr-2"
                  />
                  <div>
                    <span>Full subvolume restore</span>
                    <p className="text-xs text-muted-foreground">
                      Replace entire subvolume (requires service restart)
                    </p>
                  </div>
                </label>
              </div>
            </div>
            
            <div>
              <label htmlFor="targetPath" className="block text-sm font-medium mb-2">
                Target path
              </label>
              <input
                id="targetPath"
                type="text"
                value={targetPath}
                onChange={(e) => setTargetPath(e.target.value)}
                placeholder={restoreType === 'full' ? '/' : '/home/user/restored-files'}
                className="field"
              />
            </div>
            
            <div className="flex justify-end space-x-2">
              <button onClick={onClose} className="btn">
                Cancel
              </button>
              <button
                onClick={handleCreatePlan}
                className="btn btn-primary"
                disabled={isCreatingPlan}
              >
                {isCreatingPlan ? 'Creating the plan…' : 'Create restore plan'}
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="border border-status-warning p-4">
              <div className="flex items-start space-x-3">
                <WarningIcon className="w-5 h-5 text-status-warning-text mt-0.5" />
                <div>
                  <p className="font-medium text-status-warning-text">Restore plan</p>
                  <p className="text-sm mt-1">
                    Estimated time: {Math.round(plan.estimated_time_seconds / 60)} minutes
                  </p>
                  {plan.requires_stop && plan.requires_stop.length > 0 && (
                    <p className="text-sm mt-1">
                      Services to stop: {plan.requires_stop.join(', ')}
                    </p>
                  )}
                </div>
              </div>
            </div>
            
            <div>
              <p className="text-sm font-medium mb-2">Actions to perform:</p>
              <div className="space-y-1">
                {plan.actions.map((action: any, index: number) => (
                  <div key={index} className="text-sm flex items-center space-x-2">
                    <span className="text-muted-foreground">{index + 1}.</span>
                    <span>{action.description}</span>
                  </div>
                ))}
              </div>
            </div>
            
            <div className="flex justify-end space-x-2">
              <button
                onClick={() => setPlan(null)}
                className="btn"
              >
                Back
              </button>
              <button
                onClick={handleApplyRestore}
                className="btn btn-primary"
              >
                Apply restore
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
