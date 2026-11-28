/**
 * Conflicts Screen
 * The conflicts nOS recorded for this device that wait for a decision, as
 * a ruled list; a decision is taken in the app's own dialog.
 */

import React, {useCallback, useEffect, useState} from 'react';
import {FlatList, Pressable, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {useSyncStore} from '../../stores/syncStore';
import {ConflictResolution, FileVersion, SyncConflict} from '../../api/types';
import {errorMessage} from '../../stores/authStore';
import {ConfirmDialog} from '../../components/ConfirmDialog';
import {ChevronRightIcon} from '../../components/icons';
import {EmptyState, InfoRow, ScreenTitle} from '../../components/layout';
import {StatusMark, Working} from '../../components/pixel';
import {RefreshButton, useRefresh} from '../../components/Refresh';
import {showToast} from '../../components/Toast';
import {formatDateTime} from '../../utils/datetime';
import {countOf, fileName, formatBytes} from '../../utils/format';
import {
  metrics,
  Palette,
  type,
  usePalette,
  useThemedStyles,
} from '../../theme/theme';

const RESOLVED_WORDS: Record<ConflictResolution, string> = {
  keep_local: 'Kept the local version of',
  keep_remote: 'Kept the remote version of',
  keep_both: 'Kept both versions of',
};

function describeVersion(version: FileVersion): string {
  return `${formatBytes(version.size)}  ${formatDateTime(version.modified)}`;
}

export function ConflictsScreen(): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(conflictStyles);
  const {conflicts, conflictsError, refreshConflicts, resolveConflict} =
    useSyncStore();
  const {refreshing, refresh} = useRefresh(refreshConflicts);
  const [loaded, setLoaded] = useState(conflicts.length > 0);
  const [chosen, setChosen] = useState<SyncConflict | null>(null);
  const [resolving, setResolving] = useState(false);
  const [resolveError, setResolveError] = useState<string | null>(null);

  const unresolved = conflicts.filter(c => !c.resolved);

  useEffect(() => {
    refreshConflicts().finally(() => setLoaded(true));
  }, [refreshConflicts]);

  const choose = (conflict: SyncConflict) => {
    setResolveError(null);
    setChosen(conflict);
  };

  const handleResolve = useCallback(
    async (conflict: SyncConflict, resolution: ConflictResolution) => {
      setResolving(true);
      setResolveError(null);
      try {
        await resolveConflict(conflict.id, resolution);
        setChosen(null);
        showToast('ok', `${RESOLVED_WORDS[resolution]} ${fileName(conflict.path)}.`);
      } catch (error) {
        setResolveError(`Couldn't resolve the conflict. ${errorMessage(error)}`);
      } finally {
        setResolving(false);
      }
    },
    [resolveConflict],
  );

  const renderConflict = ({item}: {item: SyncConflict}) => (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={`${fileName(item.path)}, local ${formatBytes(
        item.local_version.size,
      )}, remote ${formatBytes(item.remote_version.size)}`}
      accessibilityHint="Choose which version to keep"
      style={({pressed}) => [styles.row, pressed && styles.rowPressed]}
      onPress={() => choose(item)}>
      <View style={styles.rowText}>
        <Text style={styles.name} numberOfLines={1}>
          {fileName(item.path)}
        </Text>
        <Text style={styles.meta} numberOfLines={1}>
          {item.path}
        </Text>
        <Text style={styles.meta}>
          {`Local ${formatBytes(item.local_version.size)}   Remote ${formatBytes(
            item.remote_version.size,
          )}`}
        </Text>
        <Text style={styles.meta}>{`Found ${formatDateTime(item.detected_at)}`}</Text>
      </View>
      <ChevronRightIcon color={p.ink2} />
    </Pressable>
  );

  const renderEmpty = () => {
    if (!loaded) {
      return (
        <View style={styles.inset}>
          <Working>Loading the conflicts</Working>
        </View>
      );
    }
    if (conflictsError) {
      return null;
    }
    return (
      <EmptyState
        title="No conflicts."
        text="A file changed in two places at once shows here, to choose which version to keep."
      />
    );
  };

  const summary =
    loaded && !conflictsError && unresolved.length > 0 ? (
      <View style={styles.inset}>
        <StatusMark state="warning">
          {`${countOf(unresolved.length, 'conflict', 'conflicts')} to resolve`}
        </StatusMark>
      </View>
    ) : null;

  return (
    <SafeAreaView edges={['top']} style={styles.screen}>
      <FlatList
        data={unresolved}
        renderItem={renderConflict}
        keyExtractor={item => item.id}
        ListHeaderComponent={
          <View style={styles.header}>
            <ScreenTitle
              title="Conflicts"
              action={<RefreshButton refreshing={refreshing} onPress={refresh} />}
            />
            {summary}
            {conflictsError ? (
              <View style={styles.inset}>
                <StatusMark state="failed">
                  {`Couldn't load the conflicts. ${conflictsError}`}
                </StatusMark>
              </View>
            ) : null}
          </View>
        }
        ListEmptyComponent={renderEmpty}
      />

      <ConfirmDialog
        visible={chosen !== null}
        title={chosen ? `Which version of ${fileName(chosen.path)} to keep?` : ''}
        message="nOS records the choice for this device. It can't be changed here afterwards."
        actions={
          chosen
            ? [
                {
                  label: 'Keep the local version',
                  onPress: () => handleResolve(chosen, 'keep_local'),
                },
                {
                  label: 'Keep the remote version',
                  onPress: () => handleResolve(chosen, 'keep_remote'),
                },
                {
                  label: 'Keep both',
                  onPress: () => handleResolve(chosen, 'keep_both'),
                },
              ]
            : []
        }
        onCancel={() => setChosen(null)}
        error={resolveError}
        busy={resolving}>
        {chosen ? (
          <View style={styles.versions}>
            <InfoRow label="Local" value={describeVersion(chosen.local_version)} />
            <InfoRow label="Remote" value={describeVersion(chosen.remote_version)} />
          </View>
        ) : null}
      </ConfirmDialog>
    </SafeAreaView>
  );
}

const conflictStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  header: {borderBottomWidth: metrics.hair, borderBottomColor: p.hairline},
  inset: {paddingHorizontal: metrics.gutter, paddingBottom: metrics.space3},
  row: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
    paddingVertical: metrics.space3,
    paddingHorizontal: metrics.gutter,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  rowPressed: {backgroundColor: p.pressed},
  rowText: {flex: 1, gap: metrics.space1},
  name: {...type.small, color: p.ink},
  meta: {...type.label, color: p.ink2},
  versions: {
    borderTopWidth: metrics.hair,
    borderTopColor: p.hairline,
    marginHorizontal: -metrics.gutter,
  },
});

export default ConflictsScreen;
