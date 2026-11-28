/// <reference types="node" />
/**
 * The Nithron identity, kept by a test where there is no lint rule for it
 * (DESIGN-APPS.md, "Motion, light and dark"): no colour value and no hue's
 * name outside the one tokens file, the faces bundled and linked, nothing
 * the identity rules out (spinners, the system's alert, shadows), and the
 * words as VOICE.md writes them.
 */
import {readdirSync, readFileSync, statSync} from 'fs';
import {join, relative} from 'path';

import {fonts} from '../src/theme/theme';

const ROOT = join(__dirname, '..');
const TOKENS_FILE = 'src/theme/nithron-tokens.js';
/** Copied whole from nithron-site: checked for colour, not for style. */
const VENDOR = 'src/vendor/';

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap(name => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      return sourceFiles(path);
    }
    return /\.(ts|tsx|js)$/.test(name) ? [path] : [];
  });
}

const FILES = [
  ...sourceFiles(join(ROOT, 'src')),
  join(ROOT, 'App.tsx'),
  join(ROOT, 'index.js'),
].map(path => ({
  name: relative(ROOT, path),
  text: readFileSync(path, 'utf8'),
}));

const APP_FILES = FILES.filter(
  f => f.name !== TOKENS_FILE && !f.name.startsWith(VENDOR),
);

/** The code without its comments (a URL's // is kept). */
function code(text: string): string {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^:'"`\\])\/\/.*$/gm, '$1');
}

/** Lines of `files` that match `pattern`, as "file: line". */
function offending(
  files: {name: string; text: string}[],
  pattern: RegExp,
  transform: (text: string) => string = t => t,
): string[] {
  return files.flatMap(({name, text}) =>
    transform(text)
      .split('\n')
      .filter(line => pattern.test(line))
      .map(line => `${name}: ${line.trim()}`),
  );
}

const HEX = /#[0-9a-f]{3,8}\b/i;
const COLOUR_FUNCTION = /\b(?:rgba?|hsla?)\(\s*\d/i;
const HUES =
  /\b(?:red|green|blue|cyan|teal|lime|yellow|gold|amber|orange|violet|purple|pink|magenta|indigo|bronze|brown|white|black|gr[ae]y)\b/i;

describe('colour lives only in the tokens file', () => {
  it('has no hex value outside it', () => {
    expect(offending(FILES.filter(f => f.name !== TOKENS_FILE), HEX)).toEqual(
      [],
    );
  });

  it('has no colour written as rgb() or hsl() with numbers', () => {
    expect(offending(APP_FILES, COLOUR_FUNCTION, code)).toEqual([]);
  });

  it("names no hue in the app's code", () => {
    expect(offending(APP_FILES, HUES, code)).toEqual([]);
  });

  it('keeps the tokens copy whole, named by package, version and licence', () => {
    const first = readFileSync(join(ROOT, TOKENS_FILE), 'utf8').split('\n')[0];
    expect(first).toMatch(/^\/\/ @nithron\/tokens 0\.4\.0 /);
    expect(first).toContain('Nithron Community License v1.0');
  });

  it('keeps the icon set whole, named by package, version and licence', () => {
    const first = readFileSync(
      join(ROOT, VENDOR, 'nithron-icons/index.js'),
      'utf8',
    ).split('\n')[0];
    expect(first).toMatch(/^\/\/ @nithron\/icons 0\.2\.0, /);
    expect(first).toContain('Nithron Community License v1.0');
  });
});

describe('the faces are bundled', () => {
  const plist = readFileSync(join(ROOT, 'ios/NithronSync/Info.plist'), 'utf8');

  it.each(Object.values(fonts))('%s is an asset on both platforms', face => {
    const file = readdirSync(join(ROOT, 'assets/fonts')).find(
      name => name.replace(/\.(otf|ttf)$/, '') === face,
    );
    expect(file).toBeDefined();
    const bundled = readFileSync(join(ROOT, 'assets/fonts', file!));
    const android = readFileSync(
      join(ROOT, 'android/app/src/main/assets/fonts', file!),
    );
    expect(android.equals(bundled)).toBe(true);
    expect(plist).toContain(`<string>${file}</string>`);
  });

  it('ships each licence beside the faces', () => {
    const names = readdirSync(join(ROOT, 'assets/fonts'));
    expect(names).toEqual(
      expect.arrayContaining([
        'MONA-SANS-LICENSE.txt',
        'MONASPACE-LICENSE.txt',
        'NITHRON-PIXEL-LICENSE.txt',
      ]),
    );
  });
});

describe('what the identity rules out', () => {
  it.each([
    ['spinner', /\bActivityIndicator\b|\bRefreshControl\b/],
    ['system alert', /\bAlert\b/],
    ['shadow or elevation', /\bshadow[A-Z]\w*\s*:|\belevation\s*:/],
    ['gradient or blur', /Gradient|BlurView|blurRadius/],
    ['second icon set', /vector-icons/],
    ['weight added to a face', /fontWeight\s*:\s*'(?!normal')/],
  ])('uses no %s', (_what, pattern) => {
    expect(offending(APP_FILES, pattern, code)).toEqual([]);
  });

  it('sets the pixel face only through PixelText', () => {
    const users = APP_FILES.filter(
      f =>
        /fonts\.pixel\b/.test(code(f.text)) &&
        f.name !== 'src/components/pixel.tsx',
    );
    expect(users.map(f => f.name)).toEqual([]);
  });
});

/** The words a person reads: string literals and JSX text. */
function words(text: string): string {
  const source = code(text);
  const literals = source.match(/'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`/g) ?? [];
  const jsxText = source.match(/>[^<>{}=]*[A-Za-z][^<>{}=]*</g) ?? [];
  return [...literals, ...jsxText].join('\n');
}

describe('the words follow VOICE.md', () => {
  it.each([
    ['em or en dash', /[—–]/],
    ['exclamation mark', /[A-Za-z]!(?:\s|$|['`<])/],
    ['"we" or "our"', /\b(?:we|our|us)\b/i],
    ['"NithronOS" for nOS', /NithronOS|\bNOS\b/],
  ])('has no %s', (_what, pattern) => {
    expect(offending(APP_FILES, pattern, words)).toEqual([]);
  });
});
