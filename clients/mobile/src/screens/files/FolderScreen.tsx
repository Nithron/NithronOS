/**
 * Folder Screen
 * Browse files within a share
 */

import React, {useEffect, useCallback, useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
  ActivityIndicator,
} from 'react-native';
import {useNavigation, useRoute, useTheme, RouteProp} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {formatDistanceToNow} from 'date-fns';

import {apiClient} from '../../api/client';
import {FileMetadata} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';

type FolderNavigationProp = NativeStackNavigationProp<FilesStackParamList, 'Folder'>;
type FolderRouteProp = RouteProp<FilesStackParamList, 'Folder'>;

export function FolderScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation<FolderNavigationProp>();
  const route = useRoute<FolderRouteProp>();
  const {shareId, path} = route.params;

  const [files, setFiles] = useState<FileMetadata[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadFiles = useCallback(async () => {
    try {
      setError(null);
      const items = await apiClient.listDirectory(shareId, path);
      // Sort: folders first, then by name
      items.sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      setFiles(items);
    } catch (err) {
      setError((err as Error).message || 'Failed to load files');
    } finally {
      setLoading(false);
    }
  }, [shareId, path]);

  useEffect(() => {
    loadFiles();
  }, [loadFiles]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    await loadFiles();
    setRefreshing(false);
  }, [loadFiles]);

  const handleFilePress = (file: FileMetadata) => {
    if (file.is_dir) {
      navigation.push('Folder', {
        shareId,
        path: file.path,
        name: file.name,
      });
    } else {
      navigation.navigate('FileDetail', {
        shareId,
        path: file.path,
      });
    }
  };

  const getFileIcon = (file: FileMetadata): string => {
    if (file.is_dir) return 'folder';
    
    const ext = file.name.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'jpg':
      case 'jpeg':
      case 'png':
      case 'gif':
      case 'webp':
        return 'file-image';
      case 'mp4':
      case 'mov':
      case 'avi':
      case 'mkv':
        return 'file-video';
      case 'mp3':
      case 'wav':
      case 'flac':
      case 'aac':
        return 'file-music';
      case 'pdf':
        return 'file-pdf-box';
      case 'doc':
      case 'docx':
        return 'file-word';
      case 'xls':
      case 'xlsx':
        return 'file-excel';
      case 'ppt':
      case 'pptx':
        return 'file-powerpoint';
      case 'zip':
      case 'rar':
      case '7z':
      case 'tar':
      case 'gz':
        return 'folder-zip';
      case 'js':
      case 'ts':
      case 'py':
      case 'go':
      case 'rs':
      case 'java':
        return 'file-code';
      default:
        return 'file-document';
    }
  };

  const getFileIconColor = (file: FileMetadata): string => {
    if (file.is_dir) return '#F59E0B';
    
    const ext = file.name.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'jpg':
      case 'jpeg':
      case 'png':
      case 'gif':
      case 'webp':
        return '#10B981';
      case 'mp4':
      case 'mov':
      case 'avi':
      case 'mkv':
        return '#8B5CF6';
      case 'mp3':
      case 'wav':
      case 'flac':
        return '#EC4899';
      case 'pdf':
        return '#EF4444';
      default:
        return colors.primary;
    }
  };

  const renderFile = ({item}: {item: FileMetadata}) => {
    const iconName = getFileIcon(item);
    const iconColor = getFileIconColor(item);

    return (
      <TouchableOpacity
        style={[styles.fileItem, {borderBottomColor: colors.border}]}
        onPress={() => handleFilePress(item)}>
        <View style={[styles.fileIcon, {backgroundColor: `${iconColor}15`}]}>
          <Icon name={iconName} size={24} color={iconColor} />
        </View>
        <View style={styles.fileContent}>
          <Text style={[styles.fileName, {color: colors.text}]} numberOfLines={1}>
            {item.name}
          </Text>
          <Text style={[styles.fileMeta, {color: `${colors.text}60`}]}>
            {item.is_dir
              ? 'Folder'
              : `${formatBytes(item.size)} • ${formatDistanceToNow(new Date(item.modified), {addSuffix: true})}`}
          </Text>
        </View>
        <Icon
          name={item.is_dir ? 'chevron-right' : 'dots-vertical'}
          size={20}
          color={`${colors.text}40`}
        />
      </TouchableOpacity>
    );
  };

  const renderEmpty = () => {
    if (loading) {
      return (
        <View style={styles.centerContainer}>
          <ActivityIndicator size="large" color={colors.primary} />
        </View>
      );
    }

    if (error) {
      return (
        <View style={styles.centerContainer}>
          <Icon name="alert-circle" size={48} color="#EF4444" />
          <Text style={[styles.errorText, {color: colors.text}]}>{error}</Text>
          <TouchableOpacity
            style={[styles.retryButton, {backgroundColor: colors.primary}]}
            onPress={loadFiles}>
            <Text style={styles.retryButtonText}>Retry</Text>
          </TouchableOpacity>
        </View>
      );
    }

    return (
      <View style={styles.centerContainer}>
        <Icon name="folder-open" size={64} color={`${colors.text}30`} />
        <Text style={[styles.emptyTitle, {color: colors.text}]}>Empty Folder</Text>
        <Text style={[styles.emptyText, {color: `${colors.text}60`}]}>
          This folder doesn't contain any files
        </Text>
      </View>
    );
  };

  return (
    <View style={[styles.container, {backgroundColor: colors.background}]}>
      {/* Breadcrumb */}
      {path !== '/' && (
        <View style={[styles.breadcrumb, {borderBottomColor: colors.border}]}>
          <Icon name="folder-outline" size={16} color={`${colors.text}60`} />
          <Text style={[styles.breadcrumbText, {color: `${colors.text}60`}]} numberOfLines={1}>
            {path}
          </Text>
        </View>
      )}

      <FlatList
        data={files}
        renderItem={renderFile}
        keyExtractor={(item) => item.path}
        ListEmptyComponent={renderEmpty}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={handleRefresh}
            tintColor={colors.primary}
          />
        }
        contentContainerStyle={files.length === 0 ? styles.emptyList : undefined}
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
  breadcrumb: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderBottomWidth: 1,
    gap: 6,
  },
  breadcrumbText: {
    fontSize: 13,
    flex: 1,
  },
  fileItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 14,
    borderBottomWidth: 1,
  },
  fileIcon: {
    width: 44,
    height: 44,
    borderRadius: 10,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  fileContent: {
    flex: 1,
  },
  fileName: {
    fontSize: 16,
    fontWeight: '500',
    marginBottom: 2,
  },
  fileMeta: {
    fontSize: 13,
  },
  centerContainer: {
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
  errorText: {
    fontSize: 15,
    marginTop: 12,
    marginBottom: 16,
    textAlign: 'center',
  },
  retryButton: {
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  retryButtonText: {
    color: '#FFFFFF',
    fontSize: 15,
    fontWeight: '600',
  },
});

export default FolderScreen;

