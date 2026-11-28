/**
 * File Detail Screen
 * A file's facts as a ruled group, and deleting it (confirmed in the app's
 * own dialog, its name typed first: the file is erased on nOS).
 */

import React, {useCallback, useEffect, useState} from 'react';
import {ScrollView, Text, View} from 'react-native';
import {RouteProp, useNavigation, useRoute} from '@react-navigation/native';

import {apiClient, ApiError} from '../../api/client';
import {FileMetadata} from '../../api/types';
import {FilesStackParamList} from '../../navigation/RootNavigator';
import {errorMessage} from '../../stores/authStore';
import {Button} from '../../components/Button';
import {ConfirmDialog} from '../../components/ConfirmDialog';
import {DeleteIcon} from '../../components/icons';
import {Group, InfoRow} from '../../components/layout';
import {StatusMark, Working} from '../../components/pixel';
import {showToast} from '../../components/Toast';
import {formatDateTime} from '../../utils/datetime';
import {fileName, formatBytes} from '../../utils/format';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';

const HTTP_FORBIDDEN = 403;
/** How much of a hash is shown; the rest is cut with an ellipsis. */
const HASH_SHOWN = 16;

type FileDetailRouteProp = RouteProp<FilesStackParamList, 'FileDetail'>;

export function FileDetailScreen(): React.JSX.Element {
  const styles = useThemedStyles(detailStyles);
  const navigation = useNavigation();
  const route = useRoute<FileDetailRouteProp>();
  const {shareId, path} = route.params;

  const [file, setFile] = useState<FileMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

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

  const name = fileName(path);

  const deleteFile = async () => {
    setIsDeleting(true);
    setDeleteError(null);
    try {
      await apiClient.deletePath(shareId, path, false);
      setConfirming(false);
      showToast('ok', `Deleted ${name}.`);
      navigation.goBack();
    } catch (err) {
      setDeleteError(
        err instanceof ApiError && err.status === HTTP_FORBIDDEN
          ? "Couldn't delete the file: you can read this share but not change it."
          : `Couldn't delete the file. ${errorMessage(err)}`,
      );
    } finally {
      setIsDeleting(false);
    }
  };

  const openConfirm = () => {
    setDeleteError(null);
    setConfirming(true);
  };

  if (loading) {
    return (
      <View style={[styles.screen, styles.inset]}>
        <Working>Loading the file</Working>
      </View>
    );
  }

  if (error || !file) {
    return (
      <View style={[styles.screen, styles.inset, styles.problem]}>
        <StatusMark state="failed">{error || 'File not found.'}</StatusMark>
        <Button kind="primary" label="Try again" onPress={loadFileDetails} />
      </View>
    );
  }

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      <Text style={styles.kind}>{fileKind(name)}</Text>

      <Group label="File">
        <InfoRow label="Size" value={formatBytes(file.size)} />
        <InfoRow label="Modified" value={formatDateTime(file.mtime)} />
        <InfoRow label="Location" value={file.path} />
        {file.hash ? (
          <InfoRow
            label="Hash"
            value={
              file.hash.length > HASH_SHOWN
                ? `${file.hash.substring(0, HASH_SHOWN)}…`
                : file.hash
            }
          />
        ) : null}
      </Group>

      <View style={styles.actions}>
        <Button
          icon={DeleteIcon}
          label={isDeleting ? 'Deleting' : 'Delete file'}
          onPress={openConfirm}
          disabled={isDeleting}
        />
      </View>

      <ConfirmDialog
        visible={confirming}
        title={`Delete ${name}?`}
        message="The file is erased from the share on nOS. It can't be restored from this phone."
        confirmName={name}
        actions={[{label: `Delete ${name}`, onPress: deleteFile}]}
        onCancel={() => setConfirming(false)}
        error={deleteError}
        busy={isDeleting}
      />
    </ScrollView>
  );
}

/** What kind of file a name says it is. */
export function fileKind(name: string): string {
  const ext = name.includes('.')
    ? name.split('.').pop()?.toLowerCase() ?? ''
    : '';
  switch (ext) {
    case '':
      return 'File';
    case 'jpg':
    case 'jpeg':
      return 'JPEG image';
    case 'png':
      return 'PNG image';
    case 'gif':
      return 'GIF image';
    case 'mp4':
      return 'MP4 video';
    case 'mov':
      return 'QuickTime video';
    case 'mp3':
      return 'MP3 audio';
    case 'pdf':
      return 'PDF document';
    case 'doc':
    case 'docx':
      return 'Word document';
    case 'xls':
    case 'xlsx':
      return 'Excel spreadsheet';
    case 'zip':
      return 'ZIP archive';
    default:
      return `${ext.toUpperCase()} file`;
  }
}

const detailStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {paddingBottom: metrics.space6},
  inset: {padding: metrics.gutter},
  problem: {gap: metrics.space4, alignItems: 'flex-start' as const},
  kind: {
    ...type.small,
    color: p.ink2,
    paddingHorizontal: metrics.gutter,
    paddingTop: metrics.space4,
  },
  actions: {paddingHorizontal: metrics.gutter, marginTop: metrics.space5},
});

export default FileDetailScreen;
