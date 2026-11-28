/**
 * Activity Screen
 * What nOS recorded for this device, as a ruled list: each entry its file,
 * what was done, and its state as a mark and a word.
 */

import React, {useEffect, useState} from 'react';
import {FlatList, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {useSyncStore} from '../../stores/syncStore';
import {ActivityAction, SyncActivity} from '../../api/types';
import {EmptyState, ScreenTitle} from '../../components/layout';
import {MarkState, Meter, StatusMark, Working} from '../../components/pixel';
import {RefreshButton, useRefresh} from '../../components/Refresh';
import {formatDateTime} from '../../utils/datetime';
import {countOf, fileName, formatBytes} from '../../utils/format';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';

const ACTION_WORDS: Record<ActivityAction, string> = {
  upload: 'Upload',
  download: 'Download',
  delete: 'Delete',
  rename: 'Rename',
  conflict: 'Conflict',
};

interface StateWords {
  state: MarkState;
  word: string;
}

function activityState(activity: SyncActivity): StateWords {
  switch (activity.status) {
    case 'pending':
      return {state: 'warning', word: 'Waiting'};
    case 'in_progress':
      return {state: 'running', word: 'Running'};
    case 'completed':
      return {
        state: 'ok',
        word: formatDateTime(activity.completed_at || activity.started_at),
      };
    case 'failed':
      return {state: 'failed', word: activity.error || 'Failed'};
    default:
      return {state: 'off', word: 'Unknown'};
  }
}

export function ActivityScreen(): React.JSX.Element {
  const styles = useThemedStyles(activityStyles);
  const {
    activities,
    activityTotal,
    activityError,
    refreshActivity,
    loadMoreActivity,
  } = useSyncStore();
  const {refreshing, refresh} = useRefresh(refreshActivity);
  const [loaded, setLoaded] = useState(activities.length > 0);

  useEffect(() => {
    refreshActivity().finally(() => setLoaded(true));
  }, [refreshActivity]);

  const renderActivity = ({item}: {item: SyncActivity}) => {
    const {state, word} = activityState(item);
    return (
      <View style={styles.row}>
        <Text style={styles.name} numberOfLines={1}>
          {fileName(item.path)}
        </Text>
        <View style={styles.facts}>
          <Text style={styles.action}>{ACTION_WORDS[item.action] ?? item.action}</Text>
          <Text style={styles.path} numberOfLines={1}>
            {item.old_path ? `${item.old_path} → ${item.path}` : item.path}
          </Text>
        </View>
        <View style={styles.state}>
          {item.status === 'in_progress' ? (
            <View style={styles.running}>
              <StatusMark state="running" textStyle={type.label}>
                {word}
              </StatusMark>
              <Meter
                kind="progress"
                value={item.progress || 0}
                label={`${fileName(item.path)}, progress`}
              />
            </View>
          ) : (
            <StatusMark state={state} textStyle={type.label}>
              {word}
            </StatusMark>
          )}
          {item.size ? <Text style={styles.size}>{formatBytes(item.size)}</Text> : null}
        </View>
      </View>
    );
  };

  const renderEmpty = () => {
    if (!loaded) {
      return (
        <View style={styles.inset}>
          <Working>Loading the activity</Working>
        </View>
      );
    }
    if (activityError) {
      return null;
    }
    return (
      <EmptyState
        title="No activity."
        text="What nOS records for this device (uploads, downloads, resolved conflicts) shows here."
      />
    );
  };

  const detail = loaded && !activityError
    ? `${countOf(activityTotal, 'entry', 'entries')} recorded for this device`
    : undefined;

  return (
    <SafeAreaView edges={['top']} style={styles.screen}>
      <FlatList
        data={activities}
        renderItem={renderActivity}
        keyExtractor={item => item.id}
        ListHeaderComponent={
          <View style={styles.header}>
            <ScreenTitle
              title="Activity"
              detail={detail}
              action={<RefreshButton refreshing={refreshing} onPress={refresh} />}
            />
            {activityError ? (
              <View style={styles.inset}>
                <StatusMark state="failed">
                  {`Couldn't load the activity. ${activityError}`}
                </StatusMark>
              </View>
            ) : null}
          </View>
        }
        ListEmptyComponent={renderEmpty}
        onEndReached={loadMoreActivity}
        onEndReachedThreshold={0.5}
      />
    </SafeAreaView>
  );
}

const activityStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  header: {borderBottomWidth: metrics.hair, borderBottomColor: p.hairline},
  inset: {paddingHorizontal: metrics.gutter, paddingBottom: metrics.space3},
  row: {
    paddingVertical: metrics.space3,
    paddingHorizontal: metrics.gutter,
    gap: metrics.space1,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  name: {...type.small, color: p.ink},
  facts: {flexDirection: 'row' as const, gap: metrics.space2},
  action: {...type.label, color: p.ink},
  path: {...type.label, color: p.ink2, flex: 1},
  state: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    justifyContent: 'space-between' as const,
    gap: metrics.space3,
    marginTop: metrics.space1,
  },
  running: {gap: metrics.space2},
  size: {...type.label, color: p.ink2},
});

export default ActivityScreen;
