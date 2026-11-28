/**
 * NithronSync on the Nithron tokens. The tokens (nithron-tokens.js, copied
 * whole from @nithron/tokens) hold every value; this file is the one place
 * where the app's names meet them, and it never holds a colour value.
 *
 * The interface has no hue: the ground, ink and its greys, and ink for the
 * current item and the first action. Colour is only status: ok (a pass),
 * warning and failed, each with its own mark. The theme follows the system
 * (useColorScheme), light or dark.
 */
import {useMemo} from 'react';
import {
  PixelRatio,
  StyleSheet,
  TextStyle,
  useColorScheme,
} from 'react-native';

import {line} from './nithron-tokens';

export type Scheme = 'light' | 'dark';

interface HueUses {
  line: string;
  text: string;
  tint: string;
  bar: string;
}

type Grounds = typeof line.light;

/** A hue in its four uses on one ground, by the name the tokens give it. */
function hue(name: string, scheme: Scheme): HueUses {
  const colours = line.colour as unknown as Record<
    string,
    Record<Scheme, HueUses>
  >;
  const entry = colours[name];
  if (!entry) {
    throw new Error(`@nithron/tokens has no colour named ${name}`);
  }
  return entry[scheme];
}

/** A colour of the tokens as red, green and blue from 0 to 255. */
function channels(value: string): [number, number, number] {
  const digits = value.replace(/^#/, '');
  return [0, 2, 4].map(i => parseInt(digits.slice(i, i + 2), 16)) as [
    number,
    number,
    number,
  ];
}

/** `share` of `top` laid over `ground`, as one opaque colour. */
function over(top: string, share: number, ground: string): string {
  const a = channels(top);
  const b = channels(ground);
  const mixed = a.map((c, i) => Math.round(c * share + b[i] * (1 - share)));
  return `rgb(${mixed[0]}, ${mixed[1]}, ${mixed[2]})`;
}

/** A colour of the tokens at an opacity. */
function faded(value: string, opacity: number): string {
  const [r, g, b] = channels(value);
  return `rgba(${r}, ${g}, ${b}, ${opacity})`;
}

/** Under a finger, a selected row: ink at 9 percent over the ground. */
const PRESSED_SHARE = 0.09;
/** What a dialog lays over the screen behind it: the ground at 70 percent. */
const SCRIM_OPACITY = 0.7;

export interface Palette {
  scheme: Scheme;
  /** The ground: screen, panel, dialog, field. */
  page: string;
  ink: string;
  /** Muted text, labels. */
  ink2: string;
  /** The quietest text: placeholders, empty blocks. */
  dim: string;
  /** Separation: the 1 px rule between rows and panels. */
  hairline: string;
  /** A line button's edge, an empty meter block. */
  rule: string;
  /** The edge of a field, a dialog, a toast. */
  edge: string;
  /** The current item, the focus ring, the part that matters (ink). */
  main: string;
  pressed: string;
  scrim: string;
  ok: string;
  okText: string;
  warning: string;
  warningText: string;
  failed: string;
  failedText: string;
}

function palette(scheme: Scheme): Palette {
  const grounds: Grounds = line[scheme];
  const pass = hue(line.status.pass, scheme);
  const earlier = hue(line.status.earlier, scheme);
  const failed = hue(line.status.failed, scheme);
  return {
    scheme,
    page: grounds.page,
    ink: grounds.ink,
    ink2: grounds['ink-2'],
    dim: grounds.dim,
    hairline: grounds.hairline,
    rule: grounds.rule,
    edge: grounds.edge,
    main: hue(line.main.studio, scheme).line,
    pressed: over(grounds.ink, PRESSED_SHARE, grounds.page),
    scrim: faded(grounds.page, SCRIM_OPACITY),
    ok: pass.line,
    okText: pass.text,
    warning: earlier.line,
    warningText: earlier.text,
    failed: failed.line,
    failedText: failed.text,
  };
}

export const palettes: Record<Scheme, Palette> = {
  light: palette('light'),
  dark: palette('dark'),
};

/** The palette of the system's theme. */
export function usePalette(): Palette {
  return useColorScheme() === 'dark' ? palettes.dark : palettes.light;
}

/**
 * Styles made from the palette, made once per theme. `factory` must be a
 * function defined once (at module level), not inside a component.
 */
export function useThemedStyles<T extends StyleSheet.NamedStyles<T>>(
  factory: (p: Palette) => T,
): T {
  const p = usePalette();
  return useMemo(() => StyleSheet.create(factory(p)), [factory, p]);
}

const ROOT_PX = 16;

/**
 * A length of the tokens in px, at the 16 px root an app keeps. A clamp()
 * takes its smallest value: the size it has on a phone's width.
 */
export function px(value: string): number {
  const clamp = /^clamp\(\s*([^,]+),/.exec(value);
  const plain = clamp ? clamp[1].trim() : value.trim();
  const amount = parseFloat(plain);
  if (Number.isNaN(amount)) {
    throw new Error(`Not a length: ${value}`);
  }
  if (plain.endsWith('rem')) {
    return amount * ROOT_PX;
  }
  if (plain.endsWith('px') || /^-?[\d.]+$/.test(plain)) {
    return amount;
  }
  throw new Error(`Not a length in px or rem: ${value}`);
}

/** A duration of the tokens (150ms, 3.6s) in milliseconds. */
export function ms(value: string): number {
  const amount = parseFloat(value);
  return value.trim().endsWith('ms') ? amount : amount * 1000;
}

/**
 * The faces, by the names of their files in assets/fonts (and the
 * PostScript names iOS finds them by). Mona Sans comes as two upstream
 * static weights: Regular, and SemiBold for the tokens' 560 (titles,
 * buttons, the current item), the nearest weight the static set has.
 * A style never adds fontWeight: each weight is its own face.
 */
export const fonts = {
  sans: 'MonaSans-Regular',
  sansStrong: 'MonaSans-SemiBold',
  mono: 'MonaspaceNeon-Regular',
  pixel: 'NithronPixel-Regular',
} as const;

type TypeName = Exclude<keyof typeof line.type, '$description'>;

function textStyle(name: TypeName, fontFamily: string): TextStyle {
  const t = line.type[name];
  const fontSize = px(t.size);
  return {
    fontFamily,
    fontSize,
    lineHeight: Math.round(fontSize * parseFloat(t.leading)),
    letterSpacing: fontSize * parseFloat(t.tracking),
  };
}

/** The lower half of the scale, as an interface takes it. Sentence case. */
export const type = {
  /** The title of a screen. */
  screenTitle: textStyle('h3', fonts.sansStrong),
  /** The title of a panel or a dialog. */
  title: textStyle('title', fonts.sansStrong),
  /** Prose a person reads. */
  body: textStyle('body', fonts.sans),
  /** Text of the interface. */
  small: textStyle('small', fonts.sans),
  /** A button, a name, the current item: the small size at 560. */
  strong: textStyle('small', fonts.sansStrong),
  /** Help under a field, small print. */
  fine: textStyle('fine', fonts.sans),
  /** Values a person copies or compares: paths, sizes, times, ids. */
  mono: textStyle('mono', fonts.mono),
  /** Labels and column heads. */
  label: textStyle('label', fonts.mono),
} satisfies Record<string, TextStyle>;

function spaceStep(step: keyof typeof line.space): number {
  return px(line.space[step]);
}

/** Device pixels: a 1 px rule stands on whole pixels of the screen. */
const wholePixels = (dp: number) =>
  Math.max(PixelRatio.roundToNearestPixel(dp), 1 / PixelRatio.get());

export const metrics = {
  space1: spaceStep('1'),
  space2: spaceStep('2'),
  space3: spaceStep('3'),
  space4: spaceStep('4'),
  space5: spaceStep('5'),
  space6: spaceStep('6'),
  /** The one left edge of a screen. */
  gutter: spaceStep('4'),
  hair: wholePixels(px(line.stroke.hair)),
  focus: wholePixels(px(line.stroke.focus)),
  /** Controls: buttons, fields. Panels stay square. */
  radiusControl: px(line.radius.control),
  radiusSmall: px(line.radius.small),
  controlHeight: px(line.layout['control-height']),
  controlHeightCompact: px(line.layout['control-height-compact']),
  /** Where a finger lands: at least 44 by 44. */
  target: px(line.layout['control-height']),
  durationState: ms(line.motion['duration-state']),
  loopLamp: ms(line.motion['loop-lamp']),
  loopCursor: ms(line.motion['loop-cursor']),
} as const;

/** React Navigation's theme on the same tokens. */
export function navigationTheme(p: Palette) {
  const regular = {fontFamily: fonts.sans, fontWeight: 'normal'} as const;
  const strong = {fontFamily: fonts.sansStrong, fontWeight: 'normal'} as const;
  return {
    dark: p.scheme === 'dark',
    colors: {
      primary: p.ink,
      background: p.page,
      card: p.page,
      text: p.ink,
      border: p.hairline,
      notification: p.ink,
    },
    fonts: {regular, medium: regular, bold: strong, heavy: strong},
  };
}
