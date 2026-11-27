/**
 * File Detail Screen
 * View file details and actions
 */

import React, {useCallback, useEffect, useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
} from 'react-native';
import {
  RouteProp,
  useNavigation,
  useRoute,
  useTheme,
} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {format} from 'date-fns';

import {apiClient, ApiError} from '../../api/client';
import {FileMetadata} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';
import {errorMessage} from '../../stores/authStore';
import {formatBytes} from '../../utils/format';

const HTTP_FORBIDDEN = 403;

type FileDetailRouteProp = RouteProp<FilesStackParamList, 'FileDetail'>;

export function FileDetailScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation();
  const route = useRoute<FileDetailRouteProp>();
  const {shareId, path} = route.params;

  const [file, setFile] = useState<FileMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const loadFileDetails = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      setFile(await apiClient.getFileMetadata(shareId, path));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [shareId, path]);

  useEffect(() => {
    loadFileDetails();
  }, [loadFileDetails]);

  const deleteFile = async () => {
    setIsDeleting(true);
    try {
      await apiClient.deletePath(shareId, path, false);
      navigation.goBack();
    } catch (err) {
      const reason =
        err instanceof ApiError && err.status === HTTP_FORBIDDEN
          ? 'You can read this share but not change it.'
          : errorMessage(err);
      Alert.alert("Couldn't delete the file", reason);
    } finally {
      setIsDeleting(false);
    }
  };

  const handleDelete = () => {
    Alert.alert(
      'Delete File',
      `Delete "${fileName(path)}" from the share? This can't be undone here.`,
      [
        {text: 'Cancel', style: 'cancel'},
        {text: 'Delete', style: 'destructive', onPress: deleteFile},
      ],
    );
  };

  if (loading) {
    return (
      <View
        style={[styles.centerContainer, {backgroundColor: colors.background}]}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  if (error || !file) {
    return (
      <View
        style={[styles.centerContainer, {backgroundColor: colors.background}]}>
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

  const name = fileName(file.path);
  const fileExt = name.includes('.')
    ? name.split('.').pop()?.toLowerCase() ?? ''
    : '';

  return (
    <ScrollView
      style={[styles.container, {backgroundColor: colors.background}]}>
      {/* File Preview */}
      <View style={[styles.previewContainer, {backgroundColor: colors.card}]}>
        <View
          style={[
            styles.fileIconLarge,
            {backgroundColor: `${colors.primary}15`},
          ]}>
          <Icon name={getFileIcon(fileExt)} size={64} color={colors.primary} />
        </View>
        <Text style={[styles.fileName, {color: colors.text}]}>{name}</Text>
        <Text style={[styles.fileType, {color: `${colors.text}60`}]}>
          {getFileType(fileExt)}
        </Text>
      </View>

      <View style={styles.actionsRow}>
        <ActionButton
          icon="delete"
          label={isDeleting ? 'Deleting…' : 'Delete'}
          onPress={handleDelete}
          disabled={isDeleting}
          colors={colors}
          destructive
        />
      </View>

      {/* File Info */}
      <View
        style={[
          styles.infoCard,
          {backgroundColor: colors.card, borderColor: colors.border},
        ]}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>
          FILE INFO
        </Text>

        <InfoRow
          icon="harddisk"
          label="Size"
          value={formatBytes(file.size)}
          colors={colors}
        />
        <InfoRow
          icon="calendar"
          label="Modified"
          value={format(new Date(file.mtime), 'MMM d, yyyy h:mm a')}
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
            value={`${file.hash.substring(0, 16)}…`}
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
  disabled?: boolean;
  colors: {primary: string; text: string; card: string};
  destructive?: boolean;
}

function ActionButton({
  icon,
  label,
  onPress,
  disabled,
  colors,
  destructive,
}: ActionButtonProps) {
  const color = destructive ? '#EF4444' : colors.primary;
  return (
    <TouchableOpacity
      accessibilityRole="button"
      accessibilityState={{disabled}}
      style={styles.actionButton}
      onPress={onPress}
      disabled={disabled}>
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
      <Icon
        name={icon}
        size={18}
        color={`${colors.text}50`}
        style={styles.infoIcon}
      />
      <Text style={[styles.infoLabel, {color: `${colors.text}70`}]}>
        {label}
      </Text>
      <Text style={[styles.infoValue, {color: colors.text}]} numberOfLines={1}>
        {value}
      </Text>
    </View>
  );
}

function fileName(path: string): string {
  return path.split('/').pop() || path;
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
