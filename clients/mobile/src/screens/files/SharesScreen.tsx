/**
 * Shares Screen
 * List all sync-enabled shares
 */

import React, {useEffect, useCallback} from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {useSyncStore} from '../../stores/syncStore';
import {SyncShare} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';

type SharesNavigationProp = NativeStackNavigationProp<FilesStackParamList, 'Shares'>;

export function SharesScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation<SharesNavigationProp>();
  const {shares, refreshShares, syncStates} = useSyncStore();
  const [refreshing, setRefreshing] = React.useState(false);

  useEffect(() => {
    refreshShares();
  }, [refreshShares]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    await refreshShares();
    setRefreshing(false);
  }, [refreshShares]);

  const handleSharePress = (share: SyncShare) => {
    navigation.navigate('Folder', {
      shareId: share.share_id,
      path: '/',
      name: share.name,
    });
  };

  const renderShare = ({item}: {item: SyncShare}) => {
    const syncState = syncStates[item.share_id];
    const pendingCount = (syncState?.pending_uploads || 0) + (syncState?.pending_downloads || 0);

    return (
      <TouchableOpacity
        style={[styles.shareItem, {backgroundColor: colors.card, borderColor: colors.border}]}
        onPress={() => handleSharePress(item)}>
        <View style={[styles.shareIcon, {backgroundColor: `${colors.primary}15`}]}>
          <Icon name="folder" size={28} color={colors.primary} />
        </View>
        <View style={styles.shareContent}>
          <Text style={[styles.shareName, {color: colors.text}]}>{item.name}</Text>
          <View style={styles.shareMeta}>
            {item.file_count !== undefined && (
              <Text style={[styles.shareMetaText, {color: `${colors.text}60`}]}>
                {item.file_count} items
              </Text>
            )}
            {item.total_size !== undefined && (
              <Text style={[styles.shareMetaText, {color: `${colors.text}60`}]}>
                • {formatBytes(item.total_size)}
              </Text>
            )}
          </View>
        </View>
        <View style={styles.shareStatus}>
          {pendingCount > 0 ? (
            <View style={[styles.pendingBadge, {backgroundColor: colors.primary}]}>
              <Text style={styles.pendingText}>{pendingCount}</Text>
            </View>
          ) : (
            <Icon name="check-circle" size={20} color="#10B981" />
          )}
          <Icon name="chevron-right" size={24} color={`${colors.text}40`} />
        </View>
      </TouchableOpacity>
    );
  };

  const renderEmpty = () => (
    <View style={styles.emptyContainer}>
      <Icon name="folder-off-outline" size={64} color={`${colors.text}30`} />
      <Text style={[styles.emptyTitle, {color: colors.text}]}>No Shares</Text>
      <Text style={[styles.emptyText, {color: `${colors.text}60`}]}>
        Enable sync on shares from your NithronOS dashboard
      </Text>
    </View>
  );

  return (
    <View style={[styles.container, {backgroundColor: colors.background}]}>
      <FlatList
        data={shares}
        renderItem={renderShare}
        keyExtractor={(item) => item.share_id}
        contentContainerStyle={styles.list}
        ListEmptyComponent={renderEmpty}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={handleRefresh}
            tintColor={colors.primary}
          />
        }
        ItemSeparatorComponent={() => <View style={styles.separator} />}
      />
    </View>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  list: {
    padding: 16,
    flexGrow: 1,
  },
  shareItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
  },
  shareIcon: {
    width: 52,
    height: 52,
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 14,
  },
  shareContent: {
    flex: 1,
  },
  shareName: {
    fontSize: 17,
    fontWeight: '600',
    marginBottom: 4,
  },
  shareMeta: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  shareMetaText: {
    fontSize: 13,
  },
  shareStatus: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  pendingBadge: {
    minWidth: 22,
    height: 22,
    borderRadius: 11,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 6,
  },
  pendingText: {
    color: '#FFFFFF',
    fontSize: 12,
    fontWeight: '600',
  },
  separator: {
    height: 12,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
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
    lineHeight: 20,
  },
});

export default SharesScreen;

