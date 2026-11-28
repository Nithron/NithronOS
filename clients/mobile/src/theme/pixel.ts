/**
 * Whole pixels for the pixel layer. A pixel icon is 16 by 16 cells and the
 * pixel face is 8 of its pixels to the em; both are sharp only when one of
 * their cells is a whole number of the screen's own pixels. A phone's
 * density is often not a whole number (2.625, 3.5), so a pixel element
 * takes the whole number of device pixels per cell nearest to its nominal
 * size: 16 dp at a density of 2 or 3, 18.3 dp (48 device pixels) at 2.625.
 */
import {PixelRatio} from 'react-native';

/** The sizes a pixel icon is shown at. */
export type IconSize = 16 | 32;
/** The sizes the pixel face is set at: whole multiples of 8 px. */
export type PixelTextSize = 16 | 24 | 32;

const ICON_CELLS = 16;
const FACE_PIXELS_PER_EM = 8;
/** The face's own line: ten of its pixels. */
const FACE_PIXELS_PER_LINE = 10;

/** Device pixels per cell nearest to `nominal` dp spread over `cells`. */
function devicePixelsPerCell(
  nominal: number,
  cells: number,
  ratio: number,
): number {
  return Math.max(1, Math.round((nominal / cells) * ratio));
}

/**
 * The side, in dp, of a grid of `cells` cells nearest to `nominal` dp in
 * which every cell is a whole number of device pixels.
 */
export function snapGrid(
  nominal: number,
  cells: number,
  ratio: number = PixelRatio.get(),
): number {
  return (devicePixelsPerCell(nominal, cells, ratio) * cells) / ratio;
}

/** The side of a pixel icon, in dp. */
export function iconSide(
  size: IconSize,
  ratio: number = PixelRatio.get(),
): number {
  return snapGrid(size, ICON_CELLS, ratio);
}

export interface PixelFontMetrics {
  fontSize: number;
  lineHeight: number;
}

/**
 * The font size and line height (in dp, before the system's text scale,
 * which React Native applies to both) at which one pixel of the face is a
 * whole number of device pixels, nearest to `size` at the reader's text
 * scale.
 */
export function pixelFont(
  size: PixelTextSize,
  ratio: number = PixelRatio.get(),
  fontScale: number = PixelRatio.getFontScale(),
): PixelFontMetrics {
  const scaled = ratio * fontScale;
  const perPixel = devicePixelsPerCell(size, FACE_PIXELS_PER_EM, scaled);
  return {
    fontSize: (perPixel * FACE_PIXELS_PER_EM) / scaled,
    lineHeight: (perPixel * FACE_PIXELS_PER_LINE) / scaled,
  };
}
