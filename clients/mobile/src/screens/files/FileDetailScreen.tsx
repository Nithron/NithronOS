/**
 * File Detail Screen
 * View file details and actions
 */

import React, {useEffect, useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
  Share,
} from 'react-native';
import {useRoute, useTheme, RouteProp} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {format} from 'date-fns';

import {apiClient} from '../../api/client';
import {FileMetadata} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';

type FileDetailRouteProp = RouteProp<FilesStackParamList, 'FileDetail'>;

export function FileDetailScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const route = useRoute<FileDetailRouteProp>();
  const {shareId, path} = route.params;

  const [file, setFile] = useState<FileMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadFileDetails();
  }, [shareId, path]);

  const loadFileDetails = async () => {
    try {
      setLoading(true);
      setError(null);
      const metadata = await apiClient.getMetadata(shareId, path);
      setFile(metadata);
    } catch (err) {
      setError((err as Error).message || 'Failed to load file details');
    } finally {
      setLoading(false);
    }
  };

  const handleShare = async () => {
    if (!file) return;
    
    try {
      const webdavUrl = `${apiClient.getServerUrl()}/dav/${shareId}${path}`;
      await Share.share({
        message: `Check out this file: ${file.name}`,
        url: webdavUrl,
      });
    } catch (err) {
      console.error('Share error:', err);
    }
  };

  const handleDownload = () => {
    // In a real implementation, this would download the file
    Alert.alert('Download', 'File download started.');
  };

  const handleDelete = () => {
    Alert.alert(
      'Delete File',
      `Are you sure you want to delete "${file?.name}"?`,
      [
        {text: 'Cancel', style: 'cancel'},
        {
          text: 'Delete',
          style: 'destructive',
          onPress: () => {
            // Would implement delete here
            Alert.alert('Deleted', 'File has been deleted.');
          },
        },
      ]
    );
  };

  if (loading) {
    return (
      <View style={[styles.centerContainer, {backgroundColor: colors.background}]}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  if (error || !file) {
    return (
      <View style={[styles.centerContainer, {backgroundColor: colors.background}]}>
        <Icon name="alert-circle" size={48} color="#EF4444" />
        <Text style={[styles.errorText, {color: colors.text}]}>
          {error || 'File not found'}
        </Text>
        <TouchableOpacity
          style={[styles.retryButton, {backgroundColor: colors.primary}]}
          onPress={loadFileDetails}>
          <Text style={styles.retryButtonText}>Retry</Text>
        </TouchableOpacity>
      </View>
    );
  }

  const fileExt = file.name.split('.').pop()?.toLowerCase() || '';

  return (
    <ScrollView style={[styles.container, {backgroundColor: colors.background}]}>
      {/* File Preview */}
      <View style={[styles.previewContainer, {backgroundColor: colors.card}]}>
        <View style={[styles.fileIconLarge, {backgroundColor: `${colors.primary}15`}]}>
          <Icon name={getFileIcon(fileExt)} size={64} color={colors.primary} />
        </View>
        <Text style={[styles.fileName, {color: colors.text}]}>{file.name}</Text>
        <Text style={[styles.fileType, {color: `${colors.text}60`}]}>
          {file.mime_type || getFileType(fileExt)}
        </Text>
      </View>

      {/* Quick Actions */}
      <View style={styles.actionsRow}>
        <ActionButton
          icon="download"
          label="Download"
          onPress={handleDownload}
          colors={colors}
        />
        <ActionButton
          icon="share-variant"
          label="Share"
          onPress={handleShare}
          colors={colors}
        />
        <ActionButton
          icon="delete"
          label="Delete"
          onPress={handleDelete}
          colors={colors}
          destructive
        />
      </View>

      {/* File Info */}
      <View style={[styles.infoCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>FILE INFO</Text>
        
        <InfoRow
          icon="harddisk"
          label="Size"
          value={formatBytes(file.size)}
          colors={colors}
        />
        <InfoRow
          icon="calendar"
          label="Modified"
          value={format(new Date(file.modified), 'MMM d, yyyy h:mm a')}
          colors={colors}
        />
        <InfoRow
          icon="folder"
          label="Location"
          value={file.path}
          colors={colors}
        />
        {file.hash && (
          <InfoRow
            icon="fingerprint"
            label="Hash"
            value={file.hash.substring(0, 16) + '...'}
            colors={colors}
          />
        )}
      </View>
    </ScrollView>
  );
}

interface ActionButtonProps {
  icon: string;
  label: string;
  onPress: () => void;
  colors: {primary: string; text: string; card: string};
  destructive?: boolean;
}

function ActionButton({icon, label, onPress, colors, destructive}: ActionButtonProps) {
  const color = destructive ? '#EF4444' : colors.primary;
  return (
    <TouchableOpacity style={styles.actionButton} onPress={onPress}>
      <View style={[styles.actionIcon, {backgroundColor: `${color}15`}]}>
        <Icon name={icon} size={22} color={color} />
      </View>
      <Text style={[styles.actionLabel, {color: colors.text}]}>{label}</Text>
    </TouchableOpacity>
  );
}

interface InfoRowProps {
  icon: string;
  label: string;
  value: string;
  colors: {text: string; primary: string};
}

function InfoRow({icon, label, value, colors}: InfoRowProps) {
  return (
    <View style={styles.infoRow}>
      <Icon name={icon} size={18} color={`${colors.text}50`} style={styles.infoIcon} />
      <Text style={[styles.infoLabel, {color: `${colors.text}70`}]}>{label}</Text>
      <Text style={[styles.infoValue, {color: colors.text}]} numberOfLines={1}>
        {value}
      </Text>
    </View>
  );
}

function getFileIcon(ext: string): string {
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
      return 'file-video';
    case 'mp3':
    case 'wav':
    case 'flac':
      return 'file-music';
    case 'pdf':
      return 'file-pdf-box';
    case 'doc':
    case 'docx':
      return 'file-word';
    case 'xls':
    case 'xlsx':
      return 'file-excel';
    case 'zip':
    case 'rar':
      return 'folder-zip';
    default:
      return 'file-document';
  }
}

function getFileType(ext: string): string {
  switch (ext) {
    case 'jpg':
    case 'jpeg':
      return 'JPEG Image';
    case 'png':
      return 'PNG Image';
    case 'gif':
      return 'GIF Image';
    case 'mp4':
      return 'MP4 Video';
    case 'mov':
      return 'QuickTime Video';
    case 'mp3':
      return 'MP3 Audio';
    case 'pdf':
      return 'PDF Document';
    case 'doc':
    case 'docx':
      return 'Word Document';
    case 'xls':
    case 'xlsx':
      return 'Excel Spreadsheet';
    case 'zip':
      return 'ZIP Archive';
    default:
      return ext.toUpperCase() + ' File';
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  centerContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
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
  previewContainer: {
    alignItems: 'center',
    padding: 32,
    marginBottom: 16,
  },
  fileIconLarge: {
    width: 120,
    height: 120,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 16,
  },
  fileName: {
    fontSize: 20,
    fontWeight: '600',
    textAlign: 'center',
    marginBottom: 4,
  },
  fileType: {
    fontSize: 14,
  },
  actionsRow: {
    flexDirection: 'row',
    justifyContent: 'center',
    paddingHorizontal: 16,
    marginBottom: 24,
    gap: 24,
  },
  actionButton: {
    alignItems: 'center',
  },
  actionIcon: {
    width: 52,
    height: 52,
    borderRadius: 26,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 6,
  },
  actionLabel: {
    fontSize: 13,
  },
  infoCard: {
    marginHorizontal: 16,
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 12,
  },
  infoRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 10,
  },
  infoIcon: {
    marginRight: 12,
  },
  infoLabel: {
    fontSize: 14,
    width: 80,
  },
  infoValue: {
    flex: 1,
    fontSize: 14,
    fontWeight: '500',
    textAlign: 'right',
  },
});

export default FileDetailScreen;

