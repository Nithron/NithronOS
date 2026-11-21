import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { HardDrive, Calendar, Plus, Trash2, RefreshCw, AlertTriangle } from 'lucide-react';
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
  
  const handleDelete = (snapshot: Snapshot) => {
    if (window.confirm(`Delete snapshot ${snapshot.path}? This cannot be undone.`)) {
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
          <h1 className="text-2xl font-semibold">Snapshots</h1>
          <p className="text-muted-foreground">
            Manage system snapshots and restore points
          </p>
        </div>
        <button
          onClick={() => setShowCreateDialog(true)}
          className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
        >
          <Plus className="w-4 h-4 mr-2" />
          Create Snapshot
        </button>
      </div>
      
      {/* Stats Cards */}
      {stats && (
        <div className="grid gap-4 md:grid-cols-3">
          <div className="bg-card rounded-lg p-4">
            <div className="flex items-center space-x-3">
              <HardDrive className="w-8 h-8 text-primary" />
              <div>
                <p className="text-sm text-muted-foreground">Total Snapshots</p>
                <p className="text-2xl font-semibold">{stats.total_count}</p>
              </div>
            </div>
          </div>
          
          <div className="bg-card rounded-lg p-4">
            <div className="flex items-center space-x-3">
              <HardDrive className="w-8 h-8 text-primary" />
              <div>
                <p className="text-sm text-muted-foreground">Total Size</p>
                <p className="text-2xl font-semibold">{formatBytes(stats.total_size_bytes)}</p>
              </div>
            </div>
          </div>
          
          <div className="bg-card rounded-lg p-4">
            <div className="flex items-center space-x-3">
              <Calendar className="w-8 h-8 text-primary" />
              <div>
                <p className="text-sm text-muted-foreground">Oldest Snapshot</p>
                <p className="text-2xl font-semibold">
                  {stats.oldest_snapshot 
                    ? formatDistanceToNow(new Date(stats.oldest_snapshot), { addSuffix: true })
                    : 'N/A'}
                </p>
              </div>
            </div>
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
            className="bg-card rounded px-3 py-1"
          >
            <option value="">All subvolumes</option>
            {subvolumes.map((subvol) => (
              <option key={subvol} value={subvol}>{subvol}</option>
            ))}
          </select>
        </div>
      )}
      
      {/* Snapshots List */}
      <div className="bg-card rounded-lg">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading snapshots...
          </div>
        ) : snapshots.length === 0 ? (
          <div className="p-8 text-center">
            <HardDrive className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground mb-4">No snapshots found</p>
            <button
              onClick={() => setShowCreateDialog(true)}
              className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
            >
              <Plus className="w-4 h-4 mr-2" />
              Create First Snapshot
            </button>
          </div>
        ) : (
          <div className="divide-y">
            {snapshots.map((snapshot) => (
              <div key={snapshot.id} className="p-4 hover:bg-muted/50 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex-1">
                    <div className="flex items-center space-x-3">
                      <HardDrive className="w-5 h-5 text-primary" />
                      <div>
                        <p className="font-medium">{snapshot.path}</p>
                        <div className="flex items-center space-x-4 mt-1 text-sm text-muted-foreground">
                          <span>Subvolume: {snapshot.subvolume}</span>
                          <span>•</span>
                          <span>
                            Created {formatDistanceToNow(new Date(snapshot.created_at), { addSuffix: true })}
                          </span>
                          {snapshot.size_bytes && (
                            <>
                              <span>•</span>
                              <span>{formatBytes(snapshot.size_bytes)}</span>
                            </>
                          )}
                          {snapshot.schedule_id && (
                            <>
                              <span>•</span>
                              <span className="text-green-500">Scheduled</span>
                            </>
                          )}
                        </div>
                        {snapshot.tags && snapshot.tags.length > 0 && (
                          <div className="flex items-center space-x-2 mt-2">
                            {snapshot.tags.map((tag: any) => (
                              <span
                                key={tag}
                                className="px-2 py-1 bg-primary/10 text-primary rounded text-xs"
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
                      className="btn btn-sm"
                      title="Restore from this snapshot"
                    >
                      <RefreshCw className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(snapshot)}
                      className="btn btn-sm text-red-500 hover:bg-red-500/10"
                      title="Delete snapshot"
                      disabled={deleteMutation.isPending}
                    >
                      <Trash2 className="w-4 h-4" />
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background rounded-lg p-6 max-w-md w-full">
        <h2 className="text-lg font-semibold mb-4">Create Snapshot</h2>
        
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">
              Select Subvolumes
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
              className="w-full bg-card rounded px-3 py-2"
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
            className="btn bg-primary text-primary-foreground"
            disabled={isCreating}
          >
            {isCreating ? 'Creating...' : 'Create Snapshot'}
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
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background rounded-lg p-6 max-w-2xl w-full max-h-[80vh] overflow-y-auto">
        <h2 className="text-lg font-semibold mb-4">Restore from Snapshot</h2>
        
        <div className="bg-card rounded p-4 mb-4">
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
                Restore Type
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
                Target Path
              </label>
              <input
                id="targetPath"
                type="text"
                value={targetPath}
                onChange={(e) => setTargetPath(e.target.value)}
                placeholder={restoreType === 'full' ? '/' : '/home/user/restored-files'}
                className="w-full bg-card rounded px-3 py-2"
              />
            </div>
            
            <div className="flex justify-end space-x-2">
              <button onClick={onClose} className="btn">
                Cancel
              </button>
              <button
                onClick={handleCreatePlan}
                className="btn bg-primary text-primary-foreground"
                disabled={isCreatingPlan}
              >
                {isCreatingPlan ? 'Creating Plan...' : 'Create Restore Plan'}
              </button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="bg-yellow-500/10 border border-yellow-500/50 rounded p-4">
              <div className="flex items-start space-x-3">
                <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5" />
                <div>
                  <p className="font-medium text-yellow-500">Restore Plan</p>
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
                className="btn bg-primary text-primary-foreground"
              >
                Apply Restore
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
