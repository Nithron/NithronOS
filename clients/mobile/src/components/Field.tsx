/**
 * A form field: its label above, the field 44 px high with a 1 px edge that
 * turns ink while it has focus, help under it, and an error at the field (a
 * cross and words in the failed colour, the edge in the failed colour).
 */
import React, {useState} from 'react';
import {Text, TextInput, TextInputProps, View} from 'react-native';

import {StatusMark} from './pixel';
import {metrics, Palette, type, usePalette, useThemedStyles} from '../theme/theme';

interface FieldProps
  extends Pick<
    TextInputProps,
    | 'value'
    | 'onChangeText'
    | 'placeholder'
    | 'maxLength'
    | 'autoCapitalize'
    | 'autoCorrect'
    | 'onSubmitEditing'
    | 'returnKeyType'
  > {
  label: string;
  help?: string;
  error?: string | null;
  /** Several lines (a pasted link). */
  multiline?: boolean;
  /** The value is something a person copies or compares: the mono face. */
  mono?: boolean;
}

/** How many lines a multiline field shows before it scrolls. */
const MULTILINE_LINES = 4;

export function Field({
  label,
  help,
  error,
  multiline,
  mono,
  ...input
}: FieldProps): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(fieldStyles);
  const [focused, setFocused] = useState(false);
  const face = mono ? type.mono : type.small;
  return (
    <View style={styles.block}>
      <Text style={styles.label}>{label}</Text>
      <TextInput
        {...input}
        accessibilityLabel={label}
        accessibilityHint={error ?? help}
        multiline={multiline}
        textAlignVertical={multiline ? 'top' : 'center'}
        placeholderTextColor={p.dim}
        selectionColor={p.ink}
        cursorColor={p.ink}
        underlineColorAndroid="transparent"
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        style={[
          styles.field,
          face,
          multiline && {
            minHeight: (face.lineHeight ?? 21) * MULTILINE_LINES + metrics.space3 * 2,
          },
          focused && styles.focused,
          error ? styles.invalid : null,
        ]}
      />
      {help && !error ? <Text style={styles.help}>{help}</Text> : null}
      {error ? (
        <View accessibilityLiveRegion="polite">
          <StatusMark state="failed" textStyle={type.fine}>
            {error}
          </StatusMark>
        </View>
      ) : null}
    </View>
  );
}

const fieldStyles = (p: Palette) => ({
  block: {gap: metrics.space2},
  label: {...type.label, color: p.ink2},
  field: {
    minHeight: metrics.controlHeight,
    borderWidth: metrics.hair,
    borderColor: p.edge,
    borderRadius: metrics.radiusControl,
    backgroundColor: p.page,
    color: p.ink,
    paddingHorizontal: metrics.space3,
    paddingVertical: metrics.space2,
  },
  focused: {borderColor: p.ink},
  invalid: {borderColor: p.failed},
  help: {...type.fine, color: p.ink2},
});
