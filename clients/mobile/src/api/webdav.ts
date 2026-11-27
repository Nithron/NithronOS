/**
 * The bits of WebDAV (/dav/{share_id}/{path}) the app uses: a folder
 * listing (PROPFIND, Depth 1) and the paths for it. nosd serves it with
 * golang.org/x/net/webdav; there is no XML parser in React Native, so the
 * multistatus is read with a small, namespace-prefix-agnostic scanner.
 */

import {FolderEntry} from './types';

/** /dav/{share}/{path} for a path relative to the share ('' = its root). */
export function davPath(shareId: string, path: string, isDir: boolean): string {
  const segments = splitPath(path).map(encodeURIComponent);
  const tail = segments.join('/');
  const slash = isDir && tail !== '' ? '/' : '';
  return `/dav/${encodeURIComponent(shareId)}/${tail}${slash}`;
}

/** A path relative to the share: no leading or trailing slash, no '.' or '..'. */
export function normalizePath(path: string): string {
  return splitPath(path).join('/');
}

export function joinPath(parent: string, name: string): string {
  return normalizePath(`${parent}/${name}`);
}

function splitPath(path: string): string[] {
  return path.split('/').filter(s => s !== '' && s !== '.' && s !== '..');
}

/**
 * The entries of a PROPFIND (Depth 1) answer for folder, without the folder
 * itself, folders first, then by name.
 */
export function parseFolderListing(
  xml: string,
  shareId: string,
  folder: string,
): FolderEntry[] {
  const base = normalizePath(folder);
  const entries: FolderEntry[] = [];
  for (const response of elements(xml, 'response')) {
    const href = firstText(response, 'href');
    if (href === null) {
      continue;
    }
    const path = sharePath(href, shareId);
    if (path === null || path === base) {
      continue;
    }
    const length = firstText(response, 'getcontentlength');
    const segments = path.split('/');
    entries.push({
      path,
      name: firstText(response, 'displayname') || segments[segments.length - 1],
      isDir: new RegExp(`<${PREFIX}collection\\b`).test(response),
      size: length === null ? 0 : Number(length) || 0,
      modified: parseHttpDate(firstText(response, 'getlastmodified')),
      contentType: firstText(response, 'getcontenttype'),
    });
  }
  return entries.sort(compareEntries);
}

function compareEntries(a: FolderEntry, b: FolderEntry): number {
  if (a.isDir !== b.isDir) {
    return a.isDir ? -1 : 1;
  }
  return a.name.localeCompare(b.name);
}

// An optional namespace prefix (D:, d:, lp1:, none).
const PREFIX = '(?:[\\w.-]+:)?';

function elements(xml: string, name: string): string[] {
  const re = new RegExp(
    `<${PREFIX}${name}\\b[^>]*>([\\s\\S]*?)</${PREFIX}${name}>`,
    'g',
  );
  return Array.from(xml.matchAll(re), m => m[1]);
}

function firstText(xml: string, name: string): string | null {
  const [inner] = elements(xml, name);
  return inner === undefined ? null : decodeEntities(inner.trim());
}

function decodeEntities(s: string): string {
  return s.replace(/&(#x[0-9a-f]+|#\d+|amp|lt|gt|quot|apos);/gi, (_, e) => {
    const lower = String(e).toLowerCase();
    if (lower.startsWith('#x')) {
      return String.fromCodePoint(parseInt(lower.slice(2), 16));
    }
    if (lower.startsWith('#')) {
      return String.fromCodePoint(parseInt(lower.slice(1), 10));
    }
    return {amp: '&', lt: '<', gt: '>', quot: '"', apos: "'"}[lower] ?? '';
  });
}

/** The share-relative path an href names, or null if it's outside the share. */
function sharePath(href: string, shareId: string): string | null {
  const pathPart = href.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/]*/i, '');
  const segments = pathPart.split('/').filter(s => s !== '');
  if (segments.length < 2 || segments[0] !== 'dav') {
    return null;
  }
  try {
    if (decodeURIComponent(segments[1]) !== shareId) {
      return null;
    }
    return normalizePath(segments.slice(2).map(decodeURIComponent).join('/'));
  } catch {
    return null;
  }
}

const MONTHS = [
  'jan',
  'feb',
  'mar',
  'apr',
  'may',
  'jun',
  'jul',
  'aug',
  'sep',
  'oct',
  'nov',
  'dec',
];

/** An HTTP date ("Fri, 27 Nov 2026 10:58:00 GMT") as ISO, or null. */
export function parseHttpDate(value: string | null): string | null {
  if (!value) {
    return null;
  }
  const m = /(\d{1,2}) ([a-z]{3}) (\d{4}) (\d{2}):(\d{2}):(\d{2})/i.exec(value);
  const month = m ? MONTHS.indexOf(m[2].toLowerCase()) : -1;
  if (!m || month < 0) {
    return null;
  }
  const time = Date.UTC(
    Number(m[3]),
    month,
    Number(m[1]),
    Number(m[4]),
    Number(m[5]),
    Number(m[6]),
  );
  return new Date(time).toISOString();
}
