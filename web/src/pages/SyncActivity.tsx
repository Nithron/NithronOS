/**
 * Sync Activity Page
 * View sync activity history and statistics
 */

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Activity,
  Upload,
  Download,
  Trash2,
  FileEdit,
  AlertTriangle,
  RefreshCw,
  CheckCircle,
  XCircle,
  Clock,
  ChevronLeft,
  ChevronRight,
  BarChart3,
  FileText,
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

import {
  listActivity,
  getActivityStats,
  listSyncShares,
  syncKeys,
  ActivityAction,
  ActivityStatus,
  formatBytes,
  formatRelativeTime,
} from '@/api/sync';

const PAGE_SIZE = 25;

export default function SyncActivityPage() {
  const [page, setPage] = useState(1);
  const [selectedShare, setSelectedShare] = useState<string>('all');

  // Fetch activity
  const {
    data: activityResponse,
    isLoading: loadingActivity,
    refetch,
  } = useQuery({
    queryKey: [...syncKeys.activity(), page, selectedShare],
    queryFn: () =>
      listActivity(
        selectedShare === 'all' ? undefined : selectedShare,
        page,
        PAGE_SIZE
      ),
    refetchInterval: 10000, // Refresh every 10 seconds
  });

  // Fetch stats
  const { data: stats, isLoading: loadingStats } = useQuery({
    queryKey: [...syncKeys.activityStats(), selectedShare],
    queryFn: () =>
      getActivityStats(selectedShare === 'all' ? undefined : selectedShare),
  });

  // Fetch shares for filter
  const { data: sharesData } = useQuery({
    queryKey: syncKeys.shares(),
    queryFn: listSyncShares,
  });

  const activities = activityResponse?.activities || [];
  const totalPages = Math.ceil((activityResponse?.total || 0) / PAGE_SIZE);

  const getActionIcon = (action: ActivityAction) => {
    switch (action) {
      case 'upload':
        return <Upload className="h-4 w-4 text-blue-500" />;
      case 'download':
        return <Download className="h-4 w-4 text-green-500" />;
      case 'delete':
        return <Trash2 className="h-4 w-4 text-red-500" />;
      case 'rename':
        return <FileEdit className="h-4 w-4 text-yellow-500" />;
      case 'conflict':
        return <AlertTriangle className="h-4 w-4 text-orange-500" />;
      default:
        return <FileText className="h-4 w-4" />;
    }
  };

  const getStatusBadge = (status: ActivityStatus) => {
    switch (status) {
      case 'completed':
        return (
          <Badge variant="secondary" className="bg-green-500/10 text-green-600">
            <CheckCircle className="h-3 w-3 mr-1" />
            Completed
          </Badge>
        );
      case 'failed':
        return (
          <Badge variant="secondary" className="bg-red-500/10 text-red-600">
            <XCircle className="h-3 w-3 mr-1" />
            Failed
          </Badge>
        );
      case 'in_progress':
        return (
          <Badge variant="secondary" className="bg-blue-500/10 text-blue-600">
            <RefreshCw className="h-3 w-3 mr-1 animate-spin" />
            In Progress
          </Badge>
        );
      case 'pending':
        return (
          <Badge variant="secondary" className="bg-yellow-500/10 text-yellow-600">
            <Clock className="h-3 w-3 mr-1" />
            Pending
          </Badge>
        );
      default:
        return <Badge variant="secondary">{status}</Badge>;
    }
  };

  const formatAction = (action: ActivityAction): string => {
    const labels: Record<ActivityAction, string> = {
      upload: 'Upload',
      download: 'Download',
      delete: 'Delete',
      rename: 'Rename',
      conflict: 'Conflict',
    };
    return labels[action] || action;
  };

  return (
    <div className="container mx-auto py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Sync Activity</h1>
          <p className="text-muted-foreground">
            Monitor file synchronization operations
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={selectedShare} onValueChange={setSelectedShare}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All shares" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Shares</SelectItem>
              {sharesData?.shares.map((share) => (
                <SelectItem key={share.share_id} value={share.share_id}>
                  {share.share_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-blue-500/10">
                <Upload className="h-5 w-5 text-blue-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">
                  {loadingStats ? '-' : stats?.uploads || 0}
                </p>
                <p className="text-sm text-muted-foreground">Uploads</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-green-500/10">
                <Download className="h-5 w-5 text-green-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">
                  {loadingStats ? '-' : stats?.downloads || 0}
                </p>
                <p className="text-sm text-muted-foreground">Downloads</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-green-500/10">
                <CheckCircle className="h-5 w-5 text-green-500" />
              </div>
              <div>
                <p className="text-2xl font-bold">
                  {loadingStats ? '-' : stats?.completed || 0}
                </p>
                <p className="text-sm text-muted-foreground">Completed</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-primary/10">
                <BarChart3 className="h-5 w-5 text-primary" />
              </div>
              <div>
                <p className="text-2xl font-bold">
                  {loadingStats ? '-' : formatBytes(stats?.bytes_synced || 0)}
                </p>
                <p className="text-sm text-muted-foreground">Total Synced</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Activity List */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Recent Activity
          </CardTitle>
          <CardDescription>
            Showing {activities.length} of {activityResponse?.total || 0} activities
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loadingActivity ? (
            <div className="space-y-3">
              {[...Array(5)].map((_, i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : activities.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <div className="p-3 rounded-full bg-muted mb-4">
                <Activity className="h-8 w-8 text-muted-foreground" />
              </div>
              <h3 className="font-semibold text-lg mb-1">No Activity</h3>
              <p className="text-muted-foreground">
                Sync activity will appear here
              </p>
            </div>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Action</TableHead>
                    <TableHead>File</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Time</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {activities.map((activity) => (
                    <TableRow key={activity.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          {getActionIcon(activity.action)}
                          <span>{formatAction(activity.action)}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="max-w-[300px]">
                          <p className="font-medium truncate">
                            {activity.path.split('/').pop()}
                          </p>
                          <p className="text-xs text-muted-foreground truncate">
                            {activity.path}
                          </p>
                        </div>
                      </TableCell>
                      <TableCell>
                        {activity.size ? formatBytes(activity.size) : '-'}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          {getStatusBadge(activity.status)}
                          {activity.status === 'in_progress' &&
                            activity.progress !== undefined && (
                              <Progress
                                value={activity.progress}
                                className="h-1 w-20"
                              />
                            )}
                          {activity.error && (
                            <p className="text-xs text-red-500 truncate max-w-[150px]">
                              {activity.error}
                            </p>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatRelativeTime(activity.started_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-between mt-4 pt-4 border-t">
                  <p className="text-sm text-muted-foreground">
                    Page {page} of {totalPages}
                  </p>
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage((p) => Math.max(1, p - 1))}
                      disabled={page === 1}
                    >
                      <ChevronLeft className="h-4 w-4 mr-1" />
                      Previous
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                      disabled={page === totalPages}
                    >
                      Next
                      <ChevronRight className="h-4 w-4 ml-1" />
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

