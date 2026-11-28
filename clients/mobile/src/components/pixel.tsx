/**
 * The pixel layer: the pixel face, state marks (the set's six state
 * icons; running blinks), meters as rows of square blocks with their
 * number, and loading as words with a block cursor. On whole pixels only,
 * left aligned, never scaled or smoothed.
 */
import React, {ReactNode} from 'react';
import {PixelRatio, StyleSheet, Text, TextStyle, View} from 'react-native';

import {
  FailedIcon,
  IconComponent,
  NoticeIcon,
  OfflineIcon,
  OkIcon,
  WarningIcon,
  WorkingIcon,
} from './icons';
import {useBlink} from '../theme/motion';
import {pixelFont, type PixelTextSize} from '../theme/pixel';
import {
  fonts,
  metrics,
  Palette,
  type,
  usePalette,
  useThemedStyles,
} from '../theme/theme';

interface PixelTextProps {
  /** 16 inside a group, 24 for a whole screen's words, 32 for the name. */
  size?: PixelTextSize;
  color?: string;
  children: ReactNode;
  accessibilityRole?: 'header' | 'text';
}

/**
 * The pixel display face (Nithron Pixel): the product's name, a screen's
 * one large number, the first sentence of an empty or loading screen.
 */
export function PixelText({
  size = 16,
  color,
  children,
  accessibilityRole,
}: PixelTextProps): React.JSX.Element {
  const p = usePalette();
  const metricsForSize = pixelFont(size);
  return (
    <Text
      accessibilityRole={accessibilityRole}
      style={[pixelStyles.face, metricsForSize, {color: color ?? p.ink}]}>
      {children}
    </Text>
  );
}

const pixelStyles = StyleSheet.create({
  face: {
    fontFamily: fonts.pixel,
    fontWeight: 'normal',
    fontStyle: 'normal',
    letterSpacing: 0,
    textAlign: 'left',
    includeFontPadding: false,
  },
});

/** The state of a thing, as a mark beside its word. */
export type MarkState = 'ok' | 'warning' | 'failed' | 'running' | 'notice' | 'off';

const markIcon: Record<MarkState, IconComponent> = {
  ok: OkIcon,
  warning: WarningIcon,
  failed: FailedIcon,
  running: WorkingIcon,
  notice: NoticeIcon,
  off: OfflineIcon,
};

function markColour(state: MarkState, p: Palette): string {
  switch (state) {
    case 'ok':
      return p.ok;
    case 'warning':
      return p.warning;
    case 'failed':
      return p.failed;
    case 'off':
      return p.ink2;
    default:
      return p.main;
  }
}

/** The colour of the words beside a mark. */
export function stateTextColour(state: MarkState, p: Palette): string {
  switch (state) {
    case 'ok':
      return p.okText;
    case 'warning':
      return p.warningText;
    case 'failed':
      return p.failedText;
    case 'off':
      return p.ink2;
    default:
      return p.ink;
  }
}

/** A state mark is a 16 px icon. */
const MARK_SIZE = 16;

/** A lamp dims to this and comes back, in steps: the square stays. */
const LAMP_DIM = 0.3;

/** A state icon at 16 px. Never alone: StatusMark puts its word beside it. */
export function Mark({state}: {state: MarkState}): React.JSX.Element {
  const p = usePalette();
  const lit = useBlink(metrics.loopLamp, state === 'running');
  const Icon = markIcon[state];
  const opacity = state === 'running' && !lit ? LAMP_DIM : 1;
  return (
    <View style={{opacity}}>
      <Icon color={markColour(state, p)} />
    </View>
  );
}

interface StatusMarkProps {
  state: MarkState;
  children: ReactNode;
  /** The words' style: the small sans face by default. */
  textStyle?: TextStyle;
  /** Words in ink rather than the state's text colour. */
  plain?: boolean;
}

/** A mark and the word that says the state. */
export function StatusMark({
  state,
  children,
  textStyle,
  plain,
}: StatusMarkProps): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(markStyles);
  const colour = plain ? p.ink : stateTextColour(state, p);
  // The mark sits in the middle of the first line of its words.
  const lineHeight = textStyle?.lineHeight ?? type.small.lineHeight ?? 0;
  const onFirstLine = {
    paddingTop: Math.max(0, Math.round((lineHeight - MARK_SIZE) / 2)),
  };
  return (
    <View style={styles.row}>
      <View style={onFirstLine}>
        <Mark state={state} />
      </View>
      <Text style={[styles.words, textStyle, {color: colour}]}>
        {children}
      </Text>
    </View>
  );
}

const markStyles = (p: Palette) => ({
  row: {
    flexDirection: 'row' as const,
    alignItems: 'flex-start' as const,
    gap: metrics.space2,
  },
  words: {
    ...type.small,
    color: p.ink,
    flexShrink: 1,
  },
});

interface MeterProps {
  value: number;
  max?: number;
  /** How many blocks the full measure has. */
  blocks?: number;
  /** progress: a task on its way; meter: a measure such as disk use. */
  kind?: 'progress' | 'meter';
  label: string;
  /** The number beside the blocks; the percentage by default. */
  valueText?: string;
}

const BLOCK = PixelRatio.roundToNearestPixel(8);
const BLOCK_GAP = PixelRatio.roundToNearestPixel(2);

/**
 * A row of square blocks that fill one by one, with its number in the
 * pixel face. A block fills only when the measure covers it whole.
 */
export function Meter({
  value,
  max = 100,
  blocks = 10,
  kind = 'meter',
  label,
  valueText,
}: MeterProps): React.JSX.Element {
  const styles = useThemedStyles(meterStyles);
  const ratio = max > 0 ? Math.min(Math.max(value / max, 0), 1) : 0;
  const filled = Math.floor(ratio * blocks);
  const text = valueText ?? `${Math.round(ratio * 100)}%`;
  return (
    <View style={styles.row}>
      <View
        accessible
        accessibilityRole={kind === 'progress' ? 'progressbar' : 'adjustable'}
        accessibilityLabel={label}
        accessibilityValue={{min: 0, max, now: value, text}}
        style={styles.blocks}>
        {Array.from({length: blocks}, (_, i) => (
          <View
            key={i}
            style={[styles.block, i < filled ? styles.full : styles.empty]}
          />
        ))}
      </View>
      <PixelText>{text}</PixelText>
    </View>
  );
}

const meterStyles = (p: Palette) => ({
  row: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
  },
  blocks: {flexDirection: 'row' as const, gap: BLOCK_GAP},
  block: {width: BLOCK, height: BLOCK},
  full: {backgroundColor: p.ink},
  empty: {borderWidth: metrics.hair, borderColor: p.rule},
});

interface WorkingProps {
  children?: string;
  size?: PixelTextSize;
}

/** Loading or working: words in the pixel face and a blinking block cursor. */
export function Working({
  children = 'Loading',
  size = 16,
}: WorkingProps): React.JSX.Element {
  const p = usePalette();
  const lit = useBlink(metrics.loopCursor);
  // One pixel of the face as drawn: the font size after the system's text
  // scale, over the eight pixels of its em. The cursor is a capital's box:
  // five pixels wide, seven high, standing on the baseline.
  const cell = (pixelFont(size).fontSize * PixelRatio.getFontScale()) / 8;
  const cursor = {
    width: cell * 5,
    height: cell * 7,
    marginLeft: cell,
    marginBottom: cell * 2,
    backgroundColor: p.ink2,
  };
  return (
    <View
      accessible
      accessibilityRole="text"
      accessibilityLabel={children}
      accessibilityLiveRegion="polite"
      style={workingStyles.row}>
      <PixelText size={size} color={p.ink2}>
        {children}
      </PixelText>
      <View style={[cursor, lit ? workingStyles.lit : workingStyles.dark]} />
    </View>
  );
}

const workingStyles = StyleSheet.create({
  row: {flexDirection: 'row', alignItems: 'flex-end'},
  lit: {opacity: 1},
  dark: {opacity: 0},
});
