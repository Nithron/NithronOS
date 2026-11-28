import {line} from '../src/theme/nithron-tokens';
import {iconSide, pixelFont, snapGrid} from '../src/theme/pixel';
import {fonts, metrics, ms, palettes, px, type} from '../src/theme/theme';
import {formatDate, formatDateTime} from '../src/utils/datetime';
import {fileKind} from '../src/screens/files/FileDetailScreen';

/** A hue of the tokens in one of its uses, by the role the tokens name. */
function role(
  name: 'pass' | 'earlier' | 'failed',
  scheme: 'light' | 'dark',
): string {
  const colours = line.colour as unknown as Record<
    string,
    Record<string, {line: string}>
  >;
  return colours[line.status[name]][scheme].line;
}

describe('palettes', () => {
  it.each(['light', 'dark'] as const)(
    'maps the %s ground and its greys onto the tokens',
    scheme => {
      const p = palettes[scheme];
      expect(p.page).toBe(line[scheme].page);
      expect(p.ink).toBe(line[scheme].ink);
      expect(p.ink2).toBe(line[scheme]['ink-2']);
      expect(p.hairline).toBe(line[scheme].hairline);
      expect(p.edge).toBe(line[scheme].edge);
    },
  );

  it.each(['light', 'dark'] as const)(
    'takes the %s states from the roles the tokens name',
    scheme => {
      const p = palettes[scheme];
      expect(p.ok).toBe(role('pass', scheme));
      expect(p.warning).toBe(role('earlier', scheme));
      expect(p.failed).toBe(role('failed', scheme));
    },
  );

  it('has no hue for the current item: it is ink', () => {
    expect(palettes.light.main).toBe(line.light.ink);
    expect(palettes.dark.main).toBe(line.dark.ink);
  });

  it('lays ink at 9 percent over the ground under a finger', () => {
    // #f6f7f8 under #131517 at 9 percent.
    expect(palettes.light.pressed).toBe('rgb(226, 227, 228)');
  });

  it('dims with the ground at 70 percent', () => {
    expect(palettes.dark.scrim).toBe('rgba(12, 13, 15, 0.7)');
  });
});

describe('lengths and type', () => {
  it('reads rem at a 16 px root, px as is, and a clamp at its smallest', () => {
    expect(px('0.9375rem')).toBe(15);
    expect(px('44px')).toBe(44);
    expect(px('clamp(1.25rem, 1.7vw, 1.875rem)')).toBe(20);
    expect(() => px('1.7vw')).toThrow();
  });

  it('reads durations', () => {
    expect(ms('150ms')).toBe(150);
    expect(ms('3.6s')).toBe(3600);
  });

  it('sets the interface in Mona Sans and values and labels in Monaspace Neon', () => {
    expect(type.small.fontFamily).toBe(fonts.sans);
    expect(type.screenTitle.fontFamily).toBe(fonts.sansStrong);
    expect(type.mono.fontFamily).toBe(fonts.mono);
    expect(type.label.fontFamily).toBe(fonts.mono);
    expect(type.small.fontSize).toBe(15);
    expect(type.label.fontSize).toBe(13);
  });

  it('keeps controls 44 px with 8 px corners', () => {
    expect(metrics.controlHeight).toBe(44);
    expect(metrics.radiusControl).toBe(8);
    expect(metrics.target).toBeGreaterThanOrEqual(44);
  });
});

describe('whole pixels', () => {
  it('keeps a 16 px icon at 16 dp where the density is whole', () => {
    expect(iconSide(16, 2)).toBe(16);
    expect(iconSide(32, 3)).toBe(32);
  });

  it('puts every cell of an icon on whole device pixels at any density', () => {
    for (const ratio of [1.5, 2.625, 2.75, 3.5]) {
      const side = iconSide(16, ratio);
      const perCell = (side * ratio) / 16;
      expect(Number.isInteger(Math.round(perCell * 1e9) / 1e9)).toBe(true);
    }
    // 2.625: three device pixels a cell, 48 in all.
    expect(snapGrid(16, 16, 2.625) * 2.625).toBeCloseTo(48);
  });

  it('sets the pixel face where one of its pixels is whole', () => {
    expect(pixelFont(16, 2, 1)).toEqual({fontSize: 16, lineHeight: 20});
    expect(pixelFont(24, 3, 1)).toEqual({fontSize: 24, lineHeight: 30});
    const odd = pixelFont(16, 2.625, 1);
    // Five device pixels to a pixel of the face: 40 to the em, 50 a line.
    expect(odd.fontSize * 2.625).toBeCloseTo(40);
    expect(odd.lineHeight * 2.625).toBeCloseTo(50);
  });

  it("follows the reader's text scale in whole steps", () => {
    // 16 px at a text scale of 1.5 is drawn at 24: at a density of 2 that is
    // 48 device pixels to the em, six to a pixel of the face.
    const big = pixelFont(16, 2, 1.5);
    expect(big.fontSize * 1.5 * 2).toBeCloseTo(48);
  });
});

describe('dates', () => {
  const iso = '2026-11-27T22:40:00Z';

  it("writes them in the reader's locale", () => {
    expect(formatDateTime(iso)).toBe(
      new Intl.DateTimeFormat(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
      }).format(new Date(iso)),
    );
    expect(formatDate(iso)).toBe(
      new Intl.DateTimeFormat(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      }).format(new Date(iso)),
    );
  });

  it('shows "-" for what is not a date', () => {
    expect(formatDateTime('not a date')).toBe('-');
    expect(formatDate(undefined)).toBe('-');
  });
});

describe('fileKind', () => {
  it('names a file by its extension, in sentence case', () => {
    expect(fileKind('report.pdf')).toBe('PDF document');
    expect(fileKind('a.tar.GZ')).toBe('GZ file');
  });

  it('says File for a name without an extension', () => {
    expect(fileKind('README')).toBe('File');
  });
});
