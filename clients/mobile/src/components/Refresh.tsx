/**
 * Refreshing a list: a line button with its word (no pull-to-refresh, whose
 * indicator is a spinner), and a toolbar row that carries it with a fact
 * about the list.
 */
import React, {ReactNode, useCallback, useState} from 'react';
import {Text, View} from 'react-native';

import {Button} from './Button';
import {RefreshIcon} from './icons';
import {metrics, Palette, type, useThemedStyles} from '../theme/theme';

/** Runs `load` and says whether it is still running. */
export function useRefresh(load: () => Promise<unknown>): {
  refreshing: boolean;
  refresh: () => Promise<void>;
} {
  const [refreshing, setRefreshing] = useState(false);
  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await load();
    } finally {
      setRefreshing(false);
    }
  }, [load]);
  return {refreshing, refresh};
}

interface RefreshButtonProps {
  refreshing: boolean;
  onPress: () => void;
}

export function RefreshButton({
  refreshing,
  onPress,
}: RefreshButtonProps): React.JSX.Element {
  return (
    <Button
      compact
      icon={RefreshIcon}
      label={refreshing ? 'Refreshing' : 'Refresh'}
      onPress={onPress}
      disabled={refreshing}
    />
  );
}

interface ToolbarProps {
  /** A fact about what is listed, in the mono face. */
  fact?: string;
  children?: ReactNode;
}

/** A row above a list: a fact at the left edge, actions at the end. */
export function Toolbar({fact, children}: ToolbarProps): React.JSX.Element {
  const styles = useThemedStyles(toolbarStyles);
  return (
    <View style={styles.bar}>
      <Text style={styles.fact} numberOfLines={2}>
        {fact ?? ''}
      </Text>
      {children}
    </View>
  );
}

const toolbarStyles = (p: Palette) => ({
  bar: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
    minHeight: metrics.target + metrics.space2,
    paddingHorizontal: metrics.gutter,
    paddingVertical: metrics.space2,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  fact: {...type.label, color: p.ink2, flex: 1},
});
