/**
 * Conflicts Screen
 * List and resolve sync conflicts
 */

import React, {useEffect, useCallback, useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  Alert,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {formatDistanceToNow} from 'date-fns';

import {useSyncStore} from '../../stores/syncStore';
import {SyncConflict} from '../../api/types';

export function ConflictsScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation();
  const {conflicts, refreshConflicts, resolveConflict, totalConflicts} = useSyncStore();
  const [refreshing, setRefreshing] = useState(false);
  const [resolvingId, setResolvingId] = useState<string | null>(null);

  const unresolvedConflicts = conflicts.filter(c => !c.resolved);

  useEffect(() => {
    refreshConflicts();
  }, [refreshConflicts]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    await refreshConflicts();
    setRefreshing(false);
  }, [refreshConflicts]);

  const handleResolve = useCallback(
    async (conflict: SyncConflict, resolution: 'keep_local' | 'keep_remote' | 'keep_both') => {
      setResolvingId(conflict.id);
      try {
        await resolveConflict(conflict.id, resolution);
        Alert.alert('Conflict Resolved', 'The file conflict has been resolved.');
      } catch (error) {
        Alert.alert('Error', 'Failed to resolve conflict. Please try again.');
      } finally {
        setResolvingId(null);
      }
    },
    [resolveConflict]
  );

  const showResolveOptions = (conflict: SyncConflict) => {
    const fileName = conflict.path.split('/').pop() || conflict.path;
    
    Alert.alert(
      `Resolve: ${fileName}`,
      `Local: ${formatBytes(conflict.local_version.size)}\nRemote: ${formatBytes(conflict.remote_version.size)}`,
      [
        {
          text: 'Keep Local',
          onPress: () => handleResolve(conflict, 'keep_local'),
        },
        {
          text: 'Keep Remote',
          onPress: () => handleResolve(conflict, 'keep_remote'),
        },
        {
          text: 'Keep Both',
          onPress: () => handleResolve(conflict, 'keep_both'),
        },
        {
          text: 'Cancel',
          style: 'cancel',
        },
      ]
    );
  };

  const renderConflict = ({item}: {item: SyncConflict}) => {
    const fileName = item.path.split('/').pop() || item.path;
    const isResolving = resolvingId === item.id;

    return (
      <TouchableOpacity
        style={[styles.conflictItem, {borderBottomColor: colors.border}]}
        onPress={() => showResolveOptions(item)}
        disabled={isResolving}>
        <View style={[styles.conflictIcon, {backgroundColor: '#FEF3C7'}]}>
          <Icon name="alert-circle" size={24} color="#F59E0B" />
        </View>
        <View style={styles.conflictContent}>
          <Text style={[styles.conflictName, {color: colors.text}]} numberOfLines={1}>
            {fileName}
          </Text>
          <Text style={[styles.conflictPath, {color: `${colors.text}60`}]} numberOfLines={1}>
            {item.path}
          </Text>
          <View style={styles.conflictVersions}>
            <View style={styles.versionBadge}>
              <Icon name="cellphone" size={12} color={colors.primary} />
              <Text style={[styles.versionText, {color: `${colors.text}70`}]}>
                Local: {formatBytes(item.local_version.size)}
              </Text>
            </View>
            <View style={styles.versionBadge}>
              <Icon name="cloud" size={12} color={colors.primary} />
              <Text style={[styles.versionText, {color: `${colors.text}70`}]}>
                Remote: {formatBytes(item.remote_version.size)}
              </Text>
            </View>
          </View>
        </View>
        <View style={styles.conflictMeta}>
          <Text style={[styles.conflictTime, {color: `${colors.text}50`}]}>
            {formatDistanceToNow(new Date(item.detected_at), {addSuffix: true})}
          </Text>
          <Icon name="chevron-right" size={20} color={`${colors.text}40`} />
        </View>
      </TouchableOpacity>
    );
  };

  const renderHeader = () => (
    <View style={[styles.header, {borderBottomColor: colors.border}]}>
      <View>
        <Text style={[styles.headerTitle, {color: colors.text}]}>Conflicts</Text>
        <Text style={[styles.headerSubtitle, {color: `${colors.text}60`}]}>
          {unresolvedConflicts.length === 0
            ? 'No conflicts to resolve'
            : `${unresolvedConflicts.length} conflict${unresolvedConflicts.length === 1 ? '' : 's'} need${unresolvedConflicts.length === 1 ? 's' : ''} attention`}
        </Text>
      </View>
    </View>
  );

  const renderEmpty = () => (
    <View style={styles.emptyContainer}>
      <View style={[styles.emptyIcon, {backgroundColor: `${colors.primary}15`}]}>
        <Icon name="check-circle" size={48} color={colors.primary} />
      </View>
      <Text style={[styles.emptyTitle, {color: colors.text}]}>All Clear!</Text>
      <Text style={[styles.emptyText, {color: `${colors.text}60`}]}>
        You have no file conflicts to resolve
      </Text>
    </View>
  );

  return (
    <View style={[styles.container, {backgroundColor: colors.background}]}>
      <FlatList
        data={unresolvedConflicts}
        renderItem={renderConflict}
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
        contentContainerStyle={unresolvedConflicts.length === 0 ? styles.emptyList : undefined}
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
    padding: 16,
    borderBottomWidth: 1,
  },
  headerTitle: {
    fontSize: 24,
    fontWeight: '700',
  },
  headerSubtitle: {
    fontSize: 14,
    marginTop: 4,
  },
  conflictItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    borderBottomWidth: 1,
  },
  conflictIcon: {
    width: 48,
    height: 48,
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  conflictContent: {
    flex: 1,
  },
  conflictName: {
    fontSize: 16,
    fontWeight: '600',
    marginBottom: 2,
  },
  conflictPath: {
    fontSize: 13,
    marginBottom: 6,
  },
  conflictVersions: {
    flexDirection: 'row',
    gap: 12,
  },
  versionBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  versionText: {
    fontSize: 12,
  },
  conflictMeta: {
    alignItems: 'flex-end',
  },
  conflictTime: {
    fontSize: 12,
    marginBottom: 4,
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
  emptyIcon: {
    width: 96,
    height: 96,
    borderRadius: 48,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 16,
  },
  emptyTitle: {
    fontSize: 20,
    fontWeight: '600',
    marginBottom: 8,
  },
  emptyText: {
    fontSize: 14,
    textAlign: 'center',
  },
});

export default ConflictsScreen;

