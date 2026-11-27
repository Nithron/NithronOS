/**
 * The pairing link the web interface's pairing dialog shows (as text and as
 * the QR code this app scans):
 *
 *   nos://sync?server=<URL>&token=<device token>&refresh=<refresh token>&key=<server key>&name=<device name>
 *
 * Built by web/src/lib/pairing.ts. This parser follows the desktop client's
 * (clients/sync-core/api/pairing.go): same checks, same messages.
 */

export interface Pairing {
  /** The server's address, without a trailing slash */
  serverUrl: string;
  deviceToken: string;
  /** Missing: the device must be paired again when its token expires */
  refreshToken: string | null;
  /** SHA-256 of the server's TLS key, normalized (normalizeKeyPin) */
  serverKey: string | null;
  deviceName: string | null;
}

export class PairingError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PairingError';
  }
}

const KEY_HEX_LENGTH = 64; // SHA-256

/** Accepts a pin as hex, with or without colons or spaces, any case. */
export function normalizeKeyPin(pin: string): string {
  const p = pin.trim().replace(/[: ]/g, '').toLowerCase();
  if (p.length !== KEY_HEX_LENGTH || !/^[0-9a-f]+$/.test(p)) {
    throw new PairingError(
      'the server key must be a SHA-256 in hex (64 characters)',
    );
  }
  return p;
}

/** Reads a nos://sync link. */
export function parsePairingLink(link: string): Pairing {
  const query = pairingQuery(link.trim());
  const serverUrl = (query.get('server') ?? '').replace(/\/+$/, '');
  if (serverHost(serverUrl) === null) {
    throw new PairingError('the pairing link has no server address');
  }
  const deviceToken = query.get('token') ?? '';
  if (!deviceToken.startsWith('nos_dt_')) {
    throw new PairingError('the pairing link has no device token');
  }
  const key = query.get('key') ?? '';
  return {
    serverUrl,
    deviceToken,
    refreshToken: query.get('refresh') || null,
    serverKey: key === '' ? null : normalizeKeyPin(key),
    deviceName: query.get('name') || null,
  };
}

/**
 * The host name in an http(s) URL, lowercased and without brackets or port:
 * what the TLS connection is made to, and so what a pinned key is kept
 * under. null when url isn't an http(s) URL with a plain host: no user
 * name, no characters an HTTP client might read differently (a backslash,
 * spaces, non-ASCII), so the host pinned is the host connected to.
 */
export function serverHost(url: string): string | null {
  const m =
    /^https?:\/\/(?:\[([0-9a-f:.]+)\]|([a-z0-9.-]+))(?::\d{1,5})?(?:[/?#]|$)/i.exec(
      url,
    );
  if (!m) {
    return null;
  }
  return (m[1] ?? m[2]).toLowerCase();
}

/** The query of a nos://sync link; the first value of each name wins. */
function pairingQuery(link: string): Map<string, string> {
  const m = /^nos:\/\/([^/?#]*)[^?#]*(?:\?([^#]*))?/i.exec(link);
  if (!m || m[1] !== 'sync') {
    throw new PairingError('not a pairing link: it starts with nos://sync?');
  }
  const query = new Map<string, string>();
  for (const part of (m[2] ?? '').split('&')) {
    if (part === '') {
      continue;
    }
    const eq = part.indexOf('=');
    const name = decodeQueryPart(eq < 0 ? part : part.slice(0, eq));
    const value = eq < 0 ? '' : decodeQueryPart(part.slice(eq + 1));
    if (!query.has(name)) {
      query.set(name, value);
    }
  }
  return query;
}

function decodeQueryPart(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, ' '));
  } catch {
    throw new PairingError('not a pairing link: it starts with nos://sync?');
  }
}
