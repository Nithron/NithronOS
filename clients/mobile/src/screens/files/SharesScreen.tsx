/**
 * Shares Screen
 * The shares this device may read that have sync turned on, as a ruled list.
 */

import React, {useEffect, useState} from 'react';
import {FlatList, View} from 'react-native';
import {useNavigation} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';

import {useSyncStore} from '../../stores/syncStore';
import {SyncShare} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';
import {EmptyState, Row} from '../../components/layout';
import {StatusMark, Working} from '../../components/pixel';
import {RefreshButton, Toolbar, useRefresh} from '../../components/Refresh';
import {countOf, formatBytes} from '../../utils/format';
import {metrics, Palette, useThemedStyles} from '../../theme/theme';

type SharesNavigationProp = NativeStackNavigationProp<
  FilesStackParamList,
  'Shares'
>;

export function SharesScreen(): React.JSX.Element {
  const styles = useThemedStyles(sharesStyles);
  const navigation = useNavigation<SharesNavigationProp>();
  const {shares, sharesError, refreshShares} = useSyncStore();
  const {refreshing, refresh} = useRefresh(refreshShares);
  // Until the first answer, "no shares" would not be true yet.
  const [loaded, setLoaded] = useState(shares.length > 0);

  useEffect(() => {
    refreshShares().finally(() => setLoaded(true));
  }, [refreshShares]);

  const openShare = (share: SyncShare) => {
    navigation.navigate('Folder', {
      shareId: share.share_id,
      path: '',
      name: share.share_name,
    });
  };

  const renderShare = ({item}: {item: SyncShare}) => (
    <Row
      title={item.share_name}
      detail={`${countOf(item.file_count, 'item', 'items')}  ${formatBytes(
        item.total_size,
      )}`}
      accessibilityLabel={`${item.share_name}, ${countOf(
        item.file_count,
        'item',
        'items',
      )}, ${formatBytes(item.total_size)}`}
      onPress={() => openShare(item)}
    />
  );

  const renderEmpty = () => {
    if (!loaded) {
      return (
        <View style={styles.loading}>
          <Working>Loading the shares</Working>
        </View>
      );
    }
    if (sharesError) {
      return (
        <View style={styles.problem}>
          <StatusMark state="failed">
            {`Couldn't load the shares. ${sharesError}`}
          </StatusMark>
        </View>
      );
    }
    return (
      <EmptyState
        title="No shares."
        text="No share you can read has sync turned on. A share's owner can turn it on in nOS."
      />
    );
  };

  return (
    <View style={styles.screen}>
      <Toolbar
        fact={loaded && shares.length > 0 ? countOf(shares.length, 'share', 'shares') : undefined}>
        <RefreshButton refreshing={refreshing} onPress={refresh} />
      </Toolbar>
      {sharesError && shares.length > 0 ? (
        <View style={styles.problem}>
          <StatusMark state="failed">
            {`Couldn't refresh the shares. ${sharesError}`}
          </StatusMark>
        </View>
      ) : null}
      <FlatList
        data={shares}
        renderItem={renderShare}
        keyExtractor={item => item.share_id}
        ListEmptyComponent={renderEmpty}
      />
    </View>
  );
}

const sharesStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  loading: {padding: metrics.gutter},
  problem: {
    padding: metrics.gutter,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
});

export default SharesScreen;
