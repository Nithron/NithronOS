/**
 * The two kinds of button: the screen's one first action in ink, and every
 * other action a line button. A destructive action is a line button too,
 * never red, and is confirmed in ConfirmDialog. 44 px high, 8 px corners;
 * an icon, where the set has one, stands beside the words.
 */
import React from 'react';
import {Pressable, Text, View} from 'react-native';

import {IconComponent} from './icons';
import {metrics, Palette, type, useThemedStyles} from '../theme/theme';
import {usePalette} from '../theme/theme';

export type ButtonKind = 'primary' | 'line';

interface ButtonProps {
  label: string;
  onPress: () => void;
  kind?: ButtonKind;
  icon?: IconComponent;
  disabled?: boolean;
  /** 32 px high, for a header or a row; the target stays 44 px. */
  compact?: boolean;
  /** Fills the width of its column. */
  block?: boolean;
  accessibilityHint?: string;
}

/** Stretches a compact button's target to 44 px. */
const COMPACT_SLOP =
  (metrics.target - metrics.controlHeightCompact) / 2;

export const Button = React.forwardRef<View, ButtonProps>(function Button(
  {
    label,
    onPress,
    kind = 'line',
    icon: Icon,
    disabled = false,
    compact = false,
    block = false,
    accessibilityHint,
  },
  ref,
) {
  const p = usePalette();
  const styles = useThemedStyles(buttonStyles);
  const textColour = kind === 'primary' ? p.page : p.ink;
  return (
    <Pressable
      ref={ref}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityHint={accessibilityHint}
      accessibilityState={{disabled}}
      disabled={disabled}
      onPress={onPress}
      hitSlop={compact ? COMPACT_SLOP : undefined}
      style={({pressed}) => [
        styles.base,
        compact ? styles.compact : styles.regular,
        kind === 'primary' ? styles.primary : styles.line,
        pressed && (kind === 'primary' ? styles.primaryPressed : styles.linePressed),
        block && styles.block,
        disabled && styles.disabled,
      ]}>
      {Icon?.drawn ? <Icon color={textColour} /> : null}
      <Text style={[styles.label, {color: textColour}]}>{label}</Text>
    </Pressable>
  );
});

/** A pressed first action dims a little: no hue, no shadow. */
const PRIMARY_PRESSED_OPACITY = 0.85;
const DISABLED_OPACITY = 0.5;

const buttonStyles = (p: Palette) => ({
  base: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    justifyContent: 'center' as const,
    gap: metrics.space2,
    borderRadius: metrics.radiusControl,
    alignSelf: 'flex-start' as const,
  },
  regular: {
    minHeight: metrics.controlHeight,
    paddingHorizontal: metrics.space5 - metrics.space1,
  },
  compact: {
    minHeight: metrics.controlHeightCompact,
    paddingHorizontal: metrics.space3,
  },
  primary: {backgroundColor: p.ink},
  primaryPressed: {opacity: PRIMARY_PRESSED_OPACITY},
  line: {
    borderWidth: metrics.hair,
    borderColor: p.rule,
    backgroundColor: p.page,
  },
  linePressed: {borderColor: p.ink, backgroundColor: p.pressed},
  block: {alignSelf: 'stretch' as const},
  disabled: {opacity: DISABLED_OPACITY},
  label: {...type.strong},
});
