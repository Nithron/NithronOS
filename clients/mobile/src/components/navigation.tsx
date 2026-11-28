/**
 * The navigation chrome: a stack's header (the screen's title on the one
 * left edge, Back in words with its icon, a hairline under it, no shadow)
 * and the tab bar (words with their icons beside them, the current item in
 * ink at 560 with a 2 px mark, the others in ink-2). An icon never stands
 * without its word; a row of tabs wraps rather than clipping a word.
 */
import React from 'react';
import {Pressable, Text, View} from 'react-native';
import type {BottomTabBarProps} from '@react-navigation/bottom-tabs';
import type {NativeStackHeaderProps} from '@react-navigation/native-stack';
import {useSafeAreaInsets} from 'react-native-safe-area-context';

import {BackIcon, IconComponent} from './icons';
import {
  fonts,
  metrics,
  Palette,
  type,
  usePalette,
  useThemedStyles,
} from '../theme/theme';

/** A stack's header. */
export function StackHeader({
  navigation,
  options,
  route,
  back,
}: NativeStackHeaderProps): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(chromeStyles);
  const insets = useSafeAreaInsets();
  const title =
    typeof options.title === 'string' ? options.title : route.name;
  return (
    <View style={[styles.header, {paddingTop: insets.top}]}>
      {back ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Back"
          onPress={navigation.goBack}
          style={({pressed}) => [styles.back, pressed && styles.pressed]}>
          <BackIcon color={p.ink} />
          <Text style={styles.backWord}>Back</Text>
        </Pressable>
      ) : null}
      <Text
        accessibilityRole="header"
        style={styles.title}
        numberOfLines={2}>
        {title}
      </Text>
    </View>
  );
}

export interface TabIcons {
  [routeName: string]: IconComponent;
}

/** The tab bar, given the icon of each tab by its route's name. */
export function makeTabBar(icons: TabIcons) {
  return function TabBar({
    state,
    descriptors,
    navigation,
  }: BottomTabBarProps): React.JSX.Element {
    const p = usePalette();
    const styles = useThemedStyles(chromeStyles);
    const insets = useSafeAreaInsets();
    return (
      <View
        accessibilityRole="tablist"
        style={[styles.tabBar, {paddingBottom: insets.bottom}]}>
        {state.routes.map((route, index) => {
          const {options} = descriptors[route.key];
          const current = state.index === index;
          const word =
            typeof options.tabBarLabel === 'string'
              ? options.tabBarLabel
              : options.title ?? route.name;
          const count = options.tabBarBadge;
          const Icon = icons[route.name];
          const colour = current ? p.ink : p.ink2;

          const onPress = () => {
            const event = navigation.emit({
              type: 'tabPress',
              target: route.key,
              canPreventDefault: true,
            });
            if (!current && !event.defaultPrevented) {
              navigation.navigate(route.name, route.params);
            }
          };

          return (
            <Pressable
              key={route.key}
              accessibilityRole="tab"
              accessibilityState={{selected: current}}
              accessibilityLabel={
                count !== undefined ? `${word}, ${count}` : word
              }
              onPress={onPress}
              style={({pressed}) => [styles.tab, pressed && styles.pressed]}>
              <View style={[styles.tabMark, current && styles.tabMarkOn]} />
              <View style={styles.tabInner}>
                {Icon?.drawn ? <Icon color={colour} /> : null}
                <Text
                  style={[
                    styles.tabWord,
                    current && styles.tabWordCurrent,
                    {color: colour},
                  ]}>
                  {word}
                </Text>
                {count !== undefined ? (
                  <Text style={[styles.tabCount, {color: colour}]}>
                    {count}
                  </Text>
                ) : null}
              </View>
            </Pressable>
          );
        })}
      </View>
    );
  };
}

const chromeStyles = (p: Palette) => ({
  header: {
    backgroundColor: p.page,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
    paddingHorizontal: metrics.gutter,
    paddingBottom: metrics.space3,
  },
  back: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    alignSelf: 'flex-start' as const,
    gap: metrics.space2,
    minHeight: metrics.target,
    paddingRight: metrics.space3,
  },
  backWord: {...type.small, color: p.ink},
  pressed: {backgroundColor: p.pressed},
  title: {...type.screenTitle, color: p.ink, paddingTop: metrics.space2},
  tabBar: {
    flexDirection: 'row' as const,
    flexWrap: 'wrap' as const,
    backgroundColor: p.page,
    borderTopWidth: metrics.hair,
    borderTopColor: p.hairline,
  },
  tab: {
    flexGrow: 1,
    minHeight: metrics.target + metrics.space2,
    paddingHorizontal: metrics.space2,
  },
  tabMark: {height: metrics.focus},
  tabMarkOn: {backgroundColor: p.main},
  tabInner: {
    flex: 1,
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    justifyContent: 'center' as const,
    gap: metrics.space2,
  },
  tabWord: {...type.fine},
  tabWordCurrent: {fontFamily: fonts.sansStrong},
  tabCount: {...type.label},
});
