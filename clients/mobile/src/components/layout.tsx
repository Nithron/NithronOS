/**
 * The ruled sheet: one ground, 1 px hairlines, square panels, one left
 * edge. A group is panels that belong together, ruled, rows sharing their
 * borders; a heading carries no icon; labels in the mono face.
 */
import React, {ReactNode} from 'react';
import {Pressable, Text, View} from 'react-native';

import {Button} from './Button';
import {ChevronRightIcon, IconComponent, OpenExternalIcon} from './icons';
import {PixelText} from './pixel';
import {metrics, Palette, type, usePalette, useThemedStyles} from '../theme/theme';

/** A label above a group or a field: mono, ink-2, sentence case. */
export function GroupLabel({children}: {children: string}): React.JSX.Element {
  const styles = useThemedStyles(layoutStyles);
  return (
    <Text accessibilityRole="header" style={styles.groupLabel}>
      {children}
    </Text>
  );
}

interface GroupProps {
  label?: string;
  children: ReactNode;
}

/** A ruled group: a hairline above, under and between its rows. */
export function Group({label, children}: GroupProps): React.JSX.Element {
  const styles = useThemedStyles(layoutStyles);
  return (
    <View style={styles.groupBlock}>
      {label ? <GroupLabel>{label}</GroupLabel> : null}
      <View style={styles.group}>{children}</View>
    </View>
  );
}

interface RowProps {
  title: string;
  /** A value or a fact under the title, in the mono face. */
  detail?: string;
  onPress: () => void;
  /** An external link: the external-link icon instead of the chevron. */
  external?: boolean;
  icon?: IconComponent;
  accessibilityLabel?: string;
}

/** A row that opens something: words, and a chevron at the end. */
export function Row({
  title,
  detail,
  onPress,
  external,
  icon: Icon,
  accessibilityLabel,
}: RowProps): React.JSX.Element {
  const p = usePalette();
  const styles = useThemedStyles(layoutStyles);
  const End = external ? OpenExternalIcon : ChevronRightIcon;
  return (
    <Pressable
      accessibilityRole={external ? 'link' : 'button'}
      accessibilityLabel={accessibilityLabel ?? [title, detail].filter(Boolean).join(', ')}
      onPress={onPress}
      style={({pressed}) => [styles.row, pressed && styles.rowPressed]}>
      {Icon?.drawn ? <Icon color={p.ink} /> : null}
      <View style={styles.rowText}>
        <Text style={styles.rowTitle} numberOfLines={2}>
          {title}
        </Text>
        {detail ? (
          <Text style={styles.rowDetail} numberOfLines={2}>
            {detail}
          </Text>
        ) : null}
      </View>
      <End color={p.ink2} />
    </Pressable>
  );
}

interface InfoRowProps {
  label: string;
  /** A value a person may copy or compare: set in the mono face. */
  value?: string;
  /** Or anything else, such as a status mark. */
  children?: ReactNode;
}

/** A label and its value, in one ruled row. */
export function InfoRow({label, value, children}: InfoRowProps): React.JSX.Element {
  const styles = useThemedStyles(layoutStyles);
  return (
    <View style={styles.infoRow} accessible accessibilityLabel={value ? `${label}, ${value}` : undefined}>
      <Text style={styles.infoLabel}>{label}</Text>
      <View style={styles.infoValueBox}>
        {value !== undefined ? (
          <Text style={styles.infoValue} selectable>
            {value}
          </Text>
        ) : (
          children
        )}
      </View>
    </View>
  );
}

interface ScreenTitleProps {
  title: string;
  /** One line of facts under the title, never a restatement of it. */
  detail?: string;
  /** An action for the whole screen, at the title's end. */
  action?: ReactNode;
}

/** The title of a tab's screen, where no stack header stands. */
export function ScreenTitle({title, detail, action}: ScreenTitleProps): React.JSX.Element {
  const styles = useThemedStyles(layoutStyles);
  return (
    <View style={styles.titleBlock}>
      <View style={styles.titleRow}>
        <Text accessibilityRole="header" style={styles.screenTitle}>
          {title}
        </Text>
        {action}
      </View>
      {detail ? <Text style={styles.titleDetail}>{detail}</Text> : null}
    </View>
  );
}

interface EmptyStateProps {
  /** The first short sentence, in the pixel face. */
  title: string;
  /** The explanation, in the sans face. */
  text?: string;
  /** The one action, if there is one. */
  actionLabel?: string;
  onAction?: () => void;
  /** Inside a group the sentence is 16 px; a whole screen's is 24. */
  inGroup?: boolean;
}

/** Nothing here: one sentence, its explanation and at most one action. */
export function EmptyState({
  title,
  text,
  actionLabel,
  onAction,
  inGroup,
}: EmptyStateProps): React.JSX.Element {
  const styles = useThemedStyles(layoutStyles);
  return (
    <View style={styles.empty}>
      <PixelText size={inGroup ? 16 : 24} accessibilityRole="header">
        {title}
      </PixelText>
      {text ? <Text style={styles.emptyText}>{text}</Text> : null}
      {actionLabel && onAction ? (
        <Button label={actionLabel} onPress={onAction} />
      ) : null}
    </View>
  );
}

/** The width of the label column of an info row. */
const LABEL_COLUMN = 112;

export const layoutStyles = (p: Palette) => ({
  groupBlock: {marginTop: metrics.space5},
  groupLabel: {
    ...type.label,
    color: p.ink2,
    paddingHorizontal: metrics.gutter,
    marginBottom: metrics.space2,
  },
  group: {
    borderTopWidth: metrics.hair,
    borderTopColor: p.hairline,
  },
  row: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    gap: metrics.space3,
    minHeight: metrics.target + metrics.space1,
    paddingVertical: metrics.space3,
    paddingHorizontal: metrics.gutter,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
    backgroundColor: p.page,
  },
  rowPressed: {backgroundColor: p.pressed},
  rowText: {flex: 1, gap: metrics.space1},
  rowTitle: {...type.small, color: p.ink},
  rowDetail: {...type.label, color: p.ink2},
  infoRow: {
    flexDirection: 'row' as const,
    alignItems: 'flex-start' as const,
    gap: metrics.space4,
    minHeight: metrics.target + metrics.space1,
    paddingVertical: metrics.space3,
    paddingHorizontal: metrics.gutter,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  infoLabel: {...type.label, color: p.ink2, width: LABEL_COLUMN, paddingTop: 1},
  infoValueBox: {flex: 1, alignItems: 'flex-end' as const},
  infoValue: {...type.mono, color: p.ink, textAlign: 'right' as const},
  titleBlock: {
    paddingHorizontal: metrics.gutter,
    paddingTop: metrics.space4,
    paddingBottom: metrics.space3,
    gap: metrics.space1,
  },
  titleRow: {
    flexDirection: 'row' as const,
    alignItems: 'center' as const,
    justifyContent: 'space-between' as const,
    gap: metrics.space3,
    flexWrap: 'wrap' as const,
  },
  screenTitle: {...type.screenTitle, color: p.ink},
  titleDetail: {...type.small, color: p.ink2},
  empty: {
    paddingHorizontal: metrics.gutter,
    paddingVertical: metrics.space6,
    gap: metrics.space3,
    alignItems: 'flex-start' as const,
  },
  emptyText: {...type.small, color: p.ink2},
});
