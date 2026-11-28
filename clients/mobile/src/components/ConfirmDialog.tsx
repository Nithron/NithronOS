/**
 * The app's own dialog for an action that cannot be taken back (never the
 * system's alert): a title that asks the question, a sentence that says
 * what happens, Cancel first (the screen reader starts on it), and buttons
 * whose words say the action. Where data is erased, the thing's name is
 * typed first. It lies on the ground with a 1 px edge and no shadow, over
 * the screen dimmed by the ground at 70 percent.
 */
import React, {ReactNode, useEffect, useRef, useState} from 'react';
import {
  AccessibilityInfo,
  findNodeHandle,
  Modal,
  ScrollView,
  Text,
  View,
} from 'react-native';

import {Button, ButtonKind} from './Button';
import {Field} from './Field';
import {StatusMark} from './pixel';
import {useReducedMotion} from '../theme/motion';
import {metrics, Palette, type, useThemedStyles} from '../theme/theme';

export interface DialogAction {
  label: string;
  onPress: () => void;
  /** Line by default: a destructive action is never filled. */
  kind?: ButtonKind;
}

interface ConfirmDialogProps {
  visible: boolean;
  /** The question. */
  title: string;
  /** What happens. */
  message: string;
  /** Facts the choice rests on, between the sentence and the buttons. */
  children?: ReactNode;
  actions: DialogAction[];
  onCancel: () => void;
  /** The name to type before an action that erases data is allowed. */
  confirmName?: string;
  /** What went wrong with the last try, shown in the dialog. */
  error?: string | null;
  /** While the action runs: every button waits. */
  busy?: boolean;
}

export function ConfirmDialog({
  visible,
  title,
  message,
  children,
  actions,
  onCancel,
  confirmName,
  error,
  busy = false,
}: ConfirmDialogProps): React.JSX.Element {
  const styles = useThemedStyles(dialogStyles);
  const reducedMotion = useReducedMotion();
  const cancelRef = useRef<View>(null);
  const [typed, setTyped] = useState('');

  useEffect(() => {
    if (!visible) {
      setTyped('');
      return;
    }
    // The screen reader starts on Cancel.
    const timer = setTimeout(() => {
      const node = findNodeHandle(cancelRef.current);
      if (node) {
        AccessibilityInfo.setAccessibilityFocus(node);
      }
    }, metrics.durationState);
    return () => clearTimeout(timer);
  }, [visible]);

  const nameTyped = confirmName === undefined || typed.trim() === confirmName;

  return (
    <Modal
      visible={visible}
      transparent
      animationType={reducedMotion ? 'none' : 'fade'}
      onRequestClose={busy ? () => undefined : onCancel}
      statusBarTranslucent>
      <View style={styles.scrim}>
        <View
          style={styles.dialog}
          accessibilityViewIsModal
          accessibilityRole="alert">
          <ScrollView
            contentContainerStyle={styles.content}
            keyboardShouldPersistTaps="handled">
            <Text accessibilityRole="header" style={styles.title}>
              {title}
            </Text>
            <Text style={styles.message}>{message}</Text>
            {children}
            {confirmName !== undefined ? (
              <Field
                label={`Type ${confirmName} to confirm`}
                value={typed}
                onChangeText={setTyped}
                autoCapitalize="none"
                autoCorrect={false}
                mono
              />
            ) : null}
            {error ? (
              <View accessibilityLiveRegion="assertive">
                <StatusMark state="failed" textStyle={type.fine}>
                  {error}
                </StatusMark>
              </View>
            ) : null}
            <View style={styles.actions}>
              <Button
                ref={cancelRef}
                label="Cancel"
                onPress={onCancel}
                disabled={busy}
                block
              />
              {actions.map(action => (
                <Button
                  key={action.label}
                  label={action.label}
                  onPress={action.onPress}
                  kind={action.kind ?? 'line'}
                  disabled={busy || !nameTyped}
                  block
                />
              ))}
            </View>
          </ScrollView>
        </View>
      </View>
    </Modal>
  );
}

/** The widest a dialog gets on a tablet. */
const DIALOG_MAX_WIDTH = 480;

const dialogStyles = (p: Palette) => ({
  scrim: {
    flex: 1,
    backgroundColor: p.scrim,
    justifyContent: 'center' as const,
    padding: metrics.gutter,
  },
  dialog: {
    alignSelf: 'center' as const,
    width: '100%' as const,
    maxWidth: DIALOG_MAX_WIDTH,
    maxHeight: '90%' as const,
    backgroundColor: p.page,
    borderWidth: metrics.hair,
    borderColor: p.edge,
  },
  content: {padding: metrics.space5, gap: metrics.space4},
  title: {...type.title, color: p.ink},
  message: {...type.small, color: p.ink},
  actions: {gap: metrics.space3, marginTop: metrics.space2},
});
