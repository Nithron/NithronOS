/**
 * Folder Screen
 * Browse files within a share
 */

import React, {useCallback, useState} from 'react';
import {FlatList, Pressable, Text, View} from 'react-native';
import {
  RouteProp,
  useFocusEffect,
  useNavigation,
  useRoute,
} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';

import {apiClient} from '../../api/client';
import {FolderEntry} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';
import {errorMessage} from '../../stores/authStore';
import {Button} from '../../components/Button';
import {ChevronRightIcon, FileIcon, FolderIcon} from '../../components/icons';
import {EmptyState} from '../../components/layout';
import {StatusMark, Working} from '../../components/pixel';
import {RefreshButton, Toolbar, useRefresh} from '../../components/Refresh';
import {formatDateTime} from '../../utils/datetime';
import {countOf, formatBytes} from '../../utils/format';
import {
  metrics,
  Palette,
  type,
  usePalette,
  useThemedStyles,
} from '../../theme/theme';

type FolderNavigationProp = NativeStackNavigationProp<
  FilesStackParamList,
  'Folder'
>;
type FolderRouteProp = RouteProp<FilesStackParamList, 'Folder'>;

export function FolderScreen(): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(folderStyles);
  const navigation = useNavigation<FolderNavigationProp>();
  const route = useRoute<FolderRouteProp>();
  const {shareId, path} = route.params;

  const [files, setFiles] = useState<FolderEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadFiles = useCallback(async () => {
    try {
      setError(null);
      // Folders first, then by name.
      setFiles(await apiClient.listFolder(shareId, path));
    } catch (err) {
      setError(errorMessage(err) || "Couldn't load the folder");
    } finally {
      setLoading(false);
    }
  }, [shareId, path]);

  const {refreshing, refresh} = useRefresh(loadFiles);

  // Again on coming back, e.g. after deleting a file.
  useFocusEffect(
    useCallback(() => {
      loadFiles();
    }, [loadFiles]),
  );

  const openEntry = (entry: FolderEntry) => {
    if (entry.isDir) {
      navigation.push('Folder', {
        shareId,
        path: entry.path,
        name: entry.name,
      });
    } else {
      navigation.navigate('FileDetail', {shareId, path: entry.path});
    }
  };

  const renderEntry = ({item}: {item: FolderEntry}) => {
    const Icon = item.isDir ? FolderIcon : FileIcon;
    return (
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`${item.name}, ${describeEntry(item)}`}
        style={({pressed}) => [styles.row, pressed && styles.rowPressed]}
        onPress={() => openEntry(item)}>
        <Icon color={p.ink} />
        <View style={styles.rowText}>
          <Text style={styles.name} numberOfLines={1}>
            {item.name}
          </Text>
          <Text style={styles.meta} numberOfLines={1}>
            {describeEntry(item)}
          </Text>
        </View>
        <ChevronRightIcon color={p.ink2} />
      </Pressable>
    );
  };

  const renderEmpty = () => {
    if (loading) {
      return (
        <View style={styles.inset}>
          <Working>Loading the folder</Working>
        </View>
      );
    }
    if (error) {
      return (
        <View style={[styles.inset, styles.problem]}>
          <StatusMark state="failed">{error}</StatusMark>
          <Button kind="primary" label="Try again" onPress={refresh} />
        </View>
      );
    }
    return (
      <EmptyState
        title="This folder is empty."
        text="Files added to it in nOS show here."
      />
    );
  };

  const fact = loading || error ? undefined : countOf(files.length, 'item', 'items');

  return (
    <View style={styles.screen}>
      {path !== '' && (
        <Text style={styles.path} numberOfLines={2} selectable>
          {path}
        </Text>
      )}
      <Toolbar fact={fact}>
        <RefreshButton refreshing={refreshing} onPress={refresh} />
      </Toolbar>
      {error && files.length > 0 ? (
        <View style={[styles.inset, styles.banner]}>
          <StatusMark state="failed">
            {`Couldn't refresh the folder. ${error}`}
          </StatusMark>
        </View>
      ) : null}
      <FlatList
        data={files}
        renderItem={renderEntry}
        keyExtractor={item => item.path}
        ListEmptyComponent={renderEmpty}
      />
    </View>
  );
}

function describeEntry(entry: FolderEntry): string {
  if (entry.isDir) {
    return 'Folder';
  }
  const size = formatBytes(entry.size);
  return entry.modified ? `${size}  ${formatDateTime(entry.modified)}` : size;
}

const folderStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  path: {
    ...type.label,
    color: p.ink2,
    paddingHorizontal: metrics.gutter,
    paddingVertical: metrics.space2,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  row: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
    minHeight: metrics.target + metrics.space1,
    paddingVertical: metrics.space3,
    paddingHorizontal: metrics.gutter,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  rowPressed: {backgroundColor: p.pressed},
  rowText: {flex: 1, gap: metrics.space1},
  name: {...type.small, color: p.ink},
  meta: {...type.label, color: p.ink2},
  inset: {padding: metrics.gutter},
  problem: {gap: metrics.space4, alignItems: 'flex-start' as const},
  banner: {borderBottomWidth: metrics.hair, borderBottomColor: p.hairline},
});

export default FolderScreen;
