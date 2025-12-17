/**
 * Activity Screen
 * Shows sync activity history
 */

import React, {useEffect, useCallback} from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  ActivityIndicator,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {formatDistanceToNow} from 'date-fns';

import {useSyncStore} from '../../stores/syncStore';
import {SyncActivity} from '../../api/types';

export function ActivityScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const {activities, refreshActivity, loadMoreActivity, status} = useSyncStore();
  const [refreshing, setRefreshing] = React.useState(false);

  useEffect(() => {
    refreshActivity();
  }, [refreshActivity]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    await refreshActivity();
    setRefreshing(false);
  }, [refreshActivity]);

  const getActivityIcon = (action: SyncActivity['action']): string => {
    switch (action) {
      case 'upload':
        return 'cloud-upload';
      case 'download':
        return 'cloud-download';
      case 'delete':
        return 'delete';
      case 'rename':
        return 'file-move';
      case 'conflict':
        return 'alert-circle';
      default:
        return 'file';
    }
  };

  const getActivityColor = (activity: SyncActivity): string => {
    if (activity.status === 'failed') return '#EF4444';
    if (activity.action === 'conflict') return '#F59E0B';
    if (activity.action === 'delete') return '#EF4444';
    return colors.primary;
  };

  const getStatusText = (activity: SyncActivity): string => {
    switch (activity.status) {
      case 'pending':
        return 'Pending';
      case 'in_progress':
        return `${activity.progress || 0}%`;
      case 'completed':
        return formatDistanceToNow(new Date(activity.completed_at || activity.started_at), {
          addSuffix: true,
        });
      case 'failed':
        return 'Failed';
      default:
        return '';
    }
  };

  const renderActivity = ({item}: {item: SyncActivity}) => {
    const iconColor = getActivityColor(item);
    const fileName = item.path.split('/').pop() || item.path;

    return (
      <View style={[styles.activityItem, {borderBottomColor: colors.border}]}>
        <View style={[styles.activityIcon, {backgroundColor: `${iconColor}15`}]}>
          <Icon name={getActivityIcon(item.action)} size={20} color={iconColor} />
        </View>
        <View style={styles.activityContent}>
          <Text style={[styles.activityName, {color: colors.text}]} numberOfLines={1}>
            {fileName}
          </Text>
          <Text style={[styles.activityPath, {color: `${colors.text}60`}]} numberOfLines={1}>
            {item.path}
          </Text>
        </View>
        <View style={styles.activityMeta}>
          <Text style={[styles.activityStatus, {color: `${colors.text}80`}]}>
            {getStatusText(item)}
          </Text>
          {item.size && (
            <Text style={[styles.activitySize, {color: `${colors.text}50`}]}>
              {formatBytes(item.size)}
            </Text>
          )}
        </View>
      </View>
    );
  };

  const renderHeader = () => (
    <View style={[styles.header, {borderBottomColor: colors.border}]}>
      <View style={styles.headerInfo}>
        <Text style={[styles.headerTitle, {color: colors.text}]}>Sync Activity</Text>
        <Text style={[styles.headerSubtitle, {color: `${colors.text}60`}]}>
          {status === 'syncing' ? 'Syncing...' : 'Up to date'}
        </Text>
      </View>
      {status === 'syncing' && <ActivityIndicator color={colors.primary} />}
    </View>
  );

  const renderEmpty = () => (
    <View style={styles.emptyContainer}>
      <Icon name="history" size={64} color={`${colors.text}30`} />
      <Text style={[styles.emptyTitle, {color: colors.text}]}>No Activity</Text>
      <Text style={[styles.emptyText, {color: `${colors.text}60`}]}>
        Your sync activity will appear here
      </Text>
    </View>
  );

  return (
    <View style={[styles.container, {backgroundColor: colors.background}]}>
      <FlatList
        data={activities}
        renderItem={renderActivity}
        keyExtractor={(item) => item.id}
        ListHeaderComponent={renderHeader}
        ListEmptyComponent={renderEmpty}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={handleRefresh}
            tintColor={colors.primary}
          />
        }
        onEndReached={loadMoreActivity}
        onEndReachedThreshold={0.5}
        contentContainerStyle={activities.length === 0 ? styles.emptyList : undefined}
      />
    </View>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: 16,
    borderBottomWidth: 1,
  },
  headerInfo: {},
  headerTitle: {
    fontSize: 24,
    fontWeight: '700',
  },
  headerSubtitle: {
    fontSize: 14,
    marginTop: 2,
  },
  activityItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    borderBottomWidth: 1,
  },
  activityIcon: {
    width: 40,
    height: 40,
    borderRadius: 10,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  activityContent: {
    flex: 1,
  },
  activityName: {
    fontSize: 15,
    fontWeight: '500',
    marginBottom: 2,
  },
  activityPath: {
    fontSize: 13,
  },
  activityMeta: {
    alignItems: 'flex-end',
  },
  activityStatus: {
    fontSize: 13,
  },
  activitySize: {
    fontSize: 12,
    marginTop: 2,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  emptyList: {
    flex: 1,
  },
  emptyTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginTop: 16,
    marginBottom: 8,
  },
  emptyText: {
    fontSize: 14,
    textAlign: 'center',
  },
});

export default ActivityScreen;

