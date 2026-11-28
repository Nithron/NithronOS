/**
 * The app's one toast: a mark and one sentence that says what happened.
 * Done leaves after about five seconds; a warning or a failure stays until
 * dismissed and says what to do. An error is also shown where it happened,
 * never in a toast alone. It lies on the ground with a 1 px edge.
 */
import React, {useEffect} from 'react';
import {AccessibilityInfo, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {create} from 'zustand';

import {Button} from './Button';
import {CloseIcon} from './icons';
import {StatusMark} from './pixel';
import {metrics, Palette, useThemedStyles} from '../theme/theme';

export type ToastState = 'ok' | 'notice' | 'warning' | 'failed';

interface ToastMessage {
  id: number;
  state: ToastState;
  text: string;
}

interface ToastStore {
  current: ToastMessage | null;
  show: (state: ToastState, text: string) => void;
  dismiss: () => void;
}

let nextId = 1;

export const useToast = create<ToastStore>(set => ({
  current: null,
  show: (state, text) => set({current: {id: nextId++, state, text}}),
  dismiss: () => set({current: null}),
}));

/** Shows a toast from anywhere (a store, a screen that is closing). */
export function showToast(state: ToastState, text: string): void {
  useToast.getState().show(state, text);
}

/** How long a done or notice toast stays. */
export const TOAST_MS = 5000;

const STAYS: ReadonlySet<ToastState> = new Set(['warning', 'failed']);

/** Where toasts appear: rendered once, above the navigation. */
export function ToastHost(): React.JSX.Element | null {
  const {current, dismiss} = useToast();
  const insets = useSafeAreaInsets();
  const styles = useThemedStyles(toastStyles);

  useEffect(() => {
    if (!current) {
      return;
    }
    AccessibilityInfo.announceForAccessibility(current.text);
    if (STAYS.has(current.state)) {
      return;
    }
    const timer = setTimeout(dismiss, TOAST_MS);
    return () => clearTimeout(timer);
  }, [current, dismiss]);

  if (!current) {
    return null;
  }

  const stays = STAYS.has(current.state);
  return (
    <View
      pointerEvents="box-none"
      style={[styles.host, {top: insets.top + metrics.space2}]}>
      <View
        style={styles.toast}
        accessibilityRole={current.state === 'failed' ? 'alert' : 'summary'}
        accessibilityLiveRegion={
          current.state === 'failed' ? 'assertive' : 'polite'
        }>
        <View style={styles.words}>
          <StatusMark state={current.state} plain>
            {current.text}
          </StatusMark>
        </View>
        {stays ? (
          <Button label="Dismiss" icon={CloseIcon} onPress={dismiss} compact />
        ) : null}
      </View>
    </View>
  );
}

const toastStyles = (p: Palette) => ({
  host: {
    position: 'absolute' as const,
    left: metrics.gutter,
    right: metrics.gutter,
  },
  toast: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
    padding: metrics.space3,
    backgroundColor: p.page,
    borderWidth: metrics.hair,
    borderColor: p.edge,
  },
  words: {flex: 1},
});
