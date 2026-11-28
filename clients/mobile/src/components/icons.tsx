/**
 * Every icon of the app, by what it means in NithronSync, drawn from the
 * shared pixel set (@nithron/icons, copied whole into src/vendor; the
 * meanings and their drawings are the nOS web interface's). Nothing else
 * draws an icon: screens import the meanings below.
 *
 * A pixel icon is one colour, drawn on 16 by 16 cells and shown at 16 or
 * 32 only, each cell on whole device pixels (theme/pixel.ts), with nothing
 * smoothed. Nothing spins. A meaning the set has no drawing for yet shows
 * nothing, so its words stand alone.
 */
import React from 'react';
import Svg, {Path} from 'react-native-svg';

import {icons as pixel, type IconName as PixelName} from '../vendor/nithron-icons';
import {iconSide, type IconSize} from '../theme/pixel';

export interface IconProps {
  /** 16 (default) or 32. */
  size?: IconSize;
  color: string;
  /** Only for an icon that stands for something by itself; else hidden. */
  accessibilityLabel?: string;
}

export type IconComponent = ((props: IconProps) => React.JSX.Element | null) & {
  displayName?: string;
  /** False for a meaning the set has no drawing for. */
  drawn: boolean;
};

/** Passed through for renderers that read it; the cells keep it sharp. */
const CRISP = {shapeRendering: 'crispEdges'} as Record<string, string>;

function pixelIcon(name: PixelName, displayName: string): IconComponent {
  function PixelIcon({size = 16, color, accessibilityLabel}: IconProps) {
    const side = iconSide(size);
    return (
      <Svg
        width={side}
        height={side}
        viewBox="0 0 16 16"
        accessible={Boolean(accessibilityLabel)}
        accessibilityLabel={accessibilityLabel}
        importantForAccessibility={
          accessibilityLabel ? 'yes' : 'no-hide-descendants'
        }
        {...CRISP}>
        <Path d={pixel[name].path} fill={color} />
      </Svg>
    );
  }
  PixelIcon.displayName = displayName;
  return Object.assign(PixelIcon, {drawn: true});
}

/** A meaning the set has no drawing for yet: nothing, its words stand alone. */
function wordsOnly(displayName: string): IconComponent {
  function NoDrawing() {
    return null;
  }
  NoDrawing.displayName = displayName;
  return Object.assign(NoDrawing, {drawn: false});
}

export const OkIcon = pixelIcon('ok', 'OkIcon');
export const WarningIcon = pixelIcon('warning', 'WarningIcon');
export const FailedIcon = pixelIcon('failed', 'FailedIcon');
export const NoticeIcon = pixelIcon('notice', 'NoticeIcon');
/** Running: StatusMark makes it blink. */
export const WorkingIcon = pixelIcon('running', 'WorkingIcon');
export const OfflineIcon = pixelIcon('off', 'OfflineIcon');

export const BackIcon = pixelIcon('back', 'BackIcon');
export const ChevronRightIcon = pixelIcon('chevron-right', 'ChevronRightIcon');
export const CloseIcon = pixelIcon('close', 'CloseIcon');
export const ConflictsIcon = wordsOnly('ConflictsIcon');
export const CopyIcon = pixelIcon('copy', 'CopyIcon');
export const DeleteIcon = pixelIcon('delete', 'DeleteIcon');
export const EditIcon = pixelIcon('edit', 'EditIcon');
export const FileIcon = pixelIcon('file', 'FileIcon');
export const FolderIcon = pixelIcon('folder', 'FolderIcon');
export const HistoryIcon = pixelIcon('history', 'HistoryIcon');
export const OpenExternalIcon = pixelIcon('external-link', 'OpenExternalIcon');
export const PairingLinkIcon = wordsOnly('PairingLinkIcon');
export const PhoneIcon = pixelIcon('device-phone', 'PhoneIcon');
export const RefreshIcon = pixelIcon('refresh', 'RefreshIcon');
export const ScanCodeIcon = wordsOnly('ScanCodeIcon');
export const ServerIcon = pixelIcon('server', 'ServerIcon');
export const SettingsIcon = pixelIcon('settings', 'SettingsIcon');
export const ShareIcon = pixelIcon('share', 'ShareIcon');
export const SignOutIcon = wordsOnly('SignOutIcon');
export const SyncIcon = pixelIcon('sync', 'SyncIcon');
