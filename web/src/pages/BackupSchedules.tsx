import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { formatDistanceToNow } from 'date-fns';
import { AddIcon, DeleteIcon, EditIcon, ScheduleIcon, StartIcon, TimeIcon, TurnedOffIcon, TurnedOnIcon } from '@/components/icons'
import { backupApi } from '@/api/backup';
import type { Schedule, ScheduleFrequency, RetentionPolicy } from '@/api/backup.types';
import { toast } from '@/components/ui/toast';

export default function BackupSchedules() {
  const queryClient = useQueryClient();
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<Schedule | null>(null);
  
  // Fetch schedules
  const { data, isLoading } = useQuery({
    queryKey: ['backup-schedules'],
    queryFn: () => backupApi.schedules.list(),
  });
  
  // Toggle enabled mutation
  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      backupApi.schedules.update(id, { enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backup-schedules'] });
      toast.success('Schedule updated');
    },
    onError: () => {
      toast.error('Failed to update schedule');
    },
  });
  
  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: (id: string) => backupApi.schedules.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backup-schedules'] });
      toast.success('Schedule deleted');
    },
    onError: () => {
      toast.error('Failed to delete schedule');
    },
  });
  
  const schedules = data?.schedules || [];
  
  const handleToggle = (schedule: Schedule) => {
    toggleMutation.mutate({ id: schedule.id, enabled: !schedule.enabled });
  };
  
  const handleDelete = (schedule: Schedule) => {
    if (window.confirm(`Delete schedule "${schedule.name}"? This will not delete existing snapshots.`)) {
      deleteMutation.mutate(schedule.id);
    }
  };
  
  const handleRunNow = async (schedule: Schedule) => {
    try {
      await backupApi.snapshots.create({ 
        subvolumes: schedule.subvolumes,
        tag: `manual-${schedule.name}`
      });
      toast.success('Backup started');
    } catch {
      toast.error('Failed to start backup');
    }
  };
  
  const formatFrequency = (freq: ScheduleFrequency) => {
    switch (freq.type) {
      case 'hourly':
        return `Every hour at :${String(freq.minute || 0).padStart(2, '0')}`;
      case 'daily':
        return `Daily at ${String(freq.hour || 0).padStart(2, '0')}:${String(freq.minute || 0).padStart(2, '0')}`;
      case 'weekly': {
        const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        return `Weekly on ${days[freq.weekday || 0]} at ${String(freq.hour || 0).padStart(2, '0')}:${String(freq.minute || 0).padStart(2, '0')}`;
      }
      case 'monthly':
        return `Monthly on day ${freq.day || 1} at ${String(freq.hour || 0).padStart(2, '0')}:${String(freq.minute || 0).padStart(2, '0')}`;
      case 'cron':
        return `Cron: ${freq.cron}`;
      default:
        return 'Unknown';
    }
  };
  
  const formatRetention = (retention: RetentionPolicy) => {
    const parts = [];
    if (retention.days > 0) parts.push(`${retention.days} daily`);
    if (retention.weeks > 0) parts.push(`${retention.weeks} weekly`);
    if (retention.months > 0) parts.push(`${retention.months} monthly`);
    if (retention.years > 0) parts.push(`${retention.years} yearly`);
    if (parts.length === 0 && retention.min_keep > 0) {
      parts.push(`Keep minimum ${retention.min_keep}`);
    }
    return parts.join(', ') || 'No retention';
  };
  
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Backup Schedules</h1>
          <p className="text-muted-foreground">
            Configure automatic backup schedules with retention policies
          </p>
        </div>
        <button
          onClick={() => setShowCreateDialog(true)}
          className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
        >
          <AddIcon className="w-4 h-4 mr-2" />
          Create Schedule
        </button>
      </div>
      
      {/* Schedules List */}
      <div className="bg-card rounded-lg">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading schedules...
          </div>
        ) : schedules.length === 0 ? (
          <div className="p-8 text-center">
            <ScheduleIcon className="w-12 h-12 mx-auto text-muted-foreground mb-4" />
            <p className="text-muted-foreground mb-4">No backup schedules configured</p>
            <button
              onClick={() => setShowCreateDialog(true)}
              className="inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2"
            >
              <AddIcon className="w-4 h-4 mr-2" />
              Create First Schedule
            </button>
          </div>
        ) : (
          <div className="divide-y">
            {schedules.map((schedule: any) => (
              <div key={schedule.id} className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center space-x-3">
                      <button
                        onClick={() => handleToggle(schedule)}
                        className="text-primary"
                        disabled={toggleMutation.isPending}
                      >
                        {schedule.enabled ? (
                          <TurnedOnIcon className="w-6 h-6" />
                        ) : (
                          <TurnedOffIcon className="w-6 h-6 text-muted-foreground" />
                        )}
                      </button>
                      <div>
                        <p className="font-medium">{schedule.name}</p>
                        <div className="flex items-center space-x-4 mt-1 text-sm text-muted-foreground">
                          <span className="flex items-center">
                            <TimeIcon className="w-3 h-3 mr-1" />
                            {formatFrequency(schedule.frequency)}
                          </span>
                          <span>•</span>
                          <span>Subvolumes: {schedule.subvolumes.join(', ')}</span>
                        </div>
                        <div className="flex items-center space-x-4 mt-1 text-sm text-muted-foreground">
                          <span>Retention: {formatRetention(schedule.retention)}</span>
                        </div>
                        <div className="flex items-center space-x-4 mt-2 text-sm">
                          {schedule.last_run && (
                            <span className="text-muted-foreground">
                              Last run: {formatDistanceToNow(new Date(schedule.last_run), { addSuffix: true })}
                            </span>
                          )}
                          {schedule.next_run && schedule.enabled && (
                            <span className="text-green-500">
                              Next run: {formatDistanceToNow(new Date(schedule.next_run), { addSuffix: true })}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => handleRunNow(schedule)}
                      className="btn btn-sm"
                      title="Run backup now"
                    >
                      <StartIcon className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => setEditingSchedule(schedule)}
                      className="btn btn-sm"
                      title="Edit schedule"
                    >
                      <EditIcon className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(schedule)}
                      className="btn btn-sm text-red-500 hover:bg-red-500/10"
                      title="Delete schedule"
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
      
      {/* Create/Edit Dialog */}
      {(showCreateDialog || editingSchedule) && (
        <ScheduleDialog
          schedule={editingSchedule}
          onClose={() => {
            setShowCreateDialog(false);
            setEditingSchedule(null);
          }}
          onSuccess={() => {
            setShowCreateDialog(false);
            setEditingSchedule(null);
            queryClient.invalidateQueries({ queryKey: ['backup-schedules'] });
          }}
        />
      )}
    </div>
  );
}

// Schedule Dialog Component
function ScheduleDialog({ 
  schedule, 
  onClose, 
  onSuccess 
}: { 
  schedule?: Schedule | null;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [name, setName] = useState(schedule?.name || '');
  const [enabled, setEnabled] = useState(schedule?.enabled ?? true);
  const [subvolumes, setSubvolumes] = useState<string[]>(schedule?.subvolumes || ['@']);
  const [frequencyType, setFrequencyType] = useState(schedule?.frequency.type || 'daily');
  const [frequency, setFrequency] = useState<ScheduleFrequency>(schedule?.frequency || {
    type: 'daily',
    hour: 2,
    minute: 0,
  });
  const [retention, setRetention] = useState<RetentionPolicy>(schedule?.retention || {
    min_keep: 3,
    days: 7,
    weeks: 4,
    months: 6,
    years: 1,
  });
  const [isSaving, setIsSaving] = useState(false);
  
  const handleSave = async () => {
    if (!name) {
      toast.error('Schedule name is required');
      return;
    }
    
    if (subvolumes.length === 0) {
      toast.error('Select at least one subvolume');
      return;
    }
    
    setIsSaving(true);
    try {
      const data = {
        name,
        enabled,
        subvolumes,
        frequency,
        retention,
        pre_hooks: [],
        post_hooks: [],
      };
      
      if (schedule) {
        await backupApi.schedules.update(schedule.id, data);
        toast.success('Schedule updated');
      } else {
        await backupApi.schedules.create(data);
        toast.success('Schedule created');
      }
      onSuccess();
    } catch {
      toast.error(schedule ? 'Failed to update schedule' : 'Failed to create schedule');
    } finally {
      setIsSaving(false);
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
      <div className="bg-background rounded-lg p-6 max-w-2xl w-full max-h-[80vh] overflow-y-auto">
        <h2 className="text-lg font-semibold mb-4">
          {schedule ? 'Edit Schedule' : 'Create Backup Schedule'}
        </h2>
        
        <div className="space-y-4">
          {/* Basic Settings */}
          <div>
            <label htmlFor="name" className="block text-sm font-medium mb-2">
              Schedule Name
            </label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., Daily System Backup"
              className="w-full bg-card rounded px-3 py-2"
            />
          </div>
          
          <div className="flex items-center">
            <input
              id="enabled"
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="mr-2"
            />
            <label htmlFor="enabled" className="text-sm">
              Enable schedule immediately
            </label>
          </div>
          
          {/* Subvolumes */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Subvolumes to Backup
            </label>
            <div className="grid grid-cols-2 gap-2">
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
          
          {/* Frequency */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Frequency
            </label>
            <select
              value={frequencyType}
              onChange={(e) => {
                const type = e.target.value as any;
                setFrequencyType(type);
                setFrequency({ ...frequency, type });
              }}
              className="w-full bg-card rounded px-3 py-2 mb-2"
            >
              <option value="hourly">Hourly</option>
              <option value="daily">Daily</option>
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="cron">Custom (Cron)</option>
            </select>
            
            {frequencyType === 'hourly' && (
              <div>
                <label className="text-sm">Minute of hour (0-59):</label>
                <input
                  type="number"
                  min="0"
                  max="59"
                  value={frequency.minute || 0}
                  onChange={(e) => setFrequency({ ...frequency, minute: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
            )}
            
            {(frequencyType === 'daily' || frequencyType === 'weekly' || frequencyType === 'monthly') && (
              <div className="flex items-center space-x-4">
                <div>
                  <label className="text-sm">Hour (0-23):</label>
                  <input
                    type="number"
                    min="0"
                    max="23"
                    value={frequency.hour || 0}
                    onChange={(e) => setFrequency({ ...frequency, hour: parseInt(e.target.value) })}
                    className="ml-2 w-20 bg-card rounded px-2 py-1"
                  />
                </div>
                <div>
                  <label className="text-sm">Minute (0-59):</label>
                  <input
                    type="number"
                    min="0"
                    max="59"
                    value={frequency.minute || 0}
                    onChange={(e) => setFrequency({ ...frequency, minute: parseInt(e.target.value) })}
                    className="ml-2 w-20 bg-card rounded px-2 py-1"
                  />
                </div>
              </div>
            )}
            
            {frequencyType === 'weekly' && (
              <div className="mt-2">
                <label className="text-sm">Day of week:</label>
                <select
                  value={frequency.weekday || 0}
                  onChange={(e) => setFrequency({ ...frequency, weekday: parseInt(e.target.value) })}
                  className="ml-2 bg-card rounded px-2 py-1"
                >
                  <option value="0">Sunday</option>
                  <option value="1">Monday</option>
                  <option value="2">Tuesday</option>
                  <option value="3">Wednesday</option>
                  <option value="4">Thursday</option>
                  <option value="5">Friday</option>
                  <option value="6">Saturday</option>
                </select>
              </div>
            )}
            
            {frequencyType === 'monthly' && (
              <div className="mt-2">
                <label className="text-sm">Day of month (1-31):</label>
                <input
                  type="number"
                  min="1"
                  max="31"
                  value={frequency.day || 1}
                  onChange={(e) => setFrequency({ ...frequency, day: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
            )}
            
            {frequencyType === 'cron' && (
              <div className="mt-2">
                <input
                  type="text"
                  value={frequency.cron || ''}
                  onChange={(e) => setFrequency({ ...frequency, cron: e.target.value })}
                  placeholder="0 2 * * *"
                  className="w-full bg-card rounded px-3 py-2"
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Standard cron expression (minute hour day month weekday)
                </p>
              </div>
            )}
          </div>
          
          {/* Retention */}
          <div>
            <label className="block text-sm font-medium mb-2">
              Retention Policy (GFS)
            </label>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-sm">Minimum to keep:</label>
                <input
                  type="number"
                  min="0"
                  value={retention.min_keep}
                  onChange={(e) => setRetention({ ...retention, min_keep: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
              <div>
                <label className="text-sm">Daily:</label>
                <input
                  type="number"
                  min="0"
                  value={retention.days}
                  onChange={(e) => setRetention({ ...retention, days: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
              <div>
                <label className="text-sm">Weekly:</label>
                <input
                  type="number"
                  min="0"
                  value={retention.weeks}
                  onChange={(e) => setRetention({ ...retention, weeks: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
              <div>
                <label className="text-sm">Monthly:</label>
                <input
                  type="number"
                  min="0"
                  value={retention.months}
                  onChange={(e) => setRetention({ ...retention, months: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
              <div>
                <label className="text-sm">Yearly:</label>
                <input
                  type="number"
                  min="0"
                  value={retention.years}
                  onChange={(e) => setRetention({ ...retention, years: parseInt(e.target.value) })}
                  className="ml-2 w-20 bg-card rounded px-2 py-1"
                />
              </div>
            </div>
            <p className="text-xs text-muted-foreground mt-2">
              Grandfather-Father-Son retention: keeps daily, weekly, monthly, and yearly snapshots
            </p>
          </div>
        </div>
        
        <div className="flex justify-end space-x-2 mt-6">
          <button
            onClick={onClose}
            className="btn"
            disabled={isSaving}
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            className="btn bg-primary text-primary-foreground"
            disabled={isSaving}
          >
            {isSaving ? 'Saving...' : (schedule ? 'Update Schedule' : 'Create Schedule')}
          </button>
        </div>
      </div>
    </div>
  );
}
