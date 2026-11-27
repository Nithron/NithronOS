import {
  normalizeKeyPin,
  PairingError,
  parsePairingLink,
  serverHost,
} from '../src/api/pairing';

// What web/src/lib/pairing.ts builds (URLSearchParams encoding).
function webLink(params: Record<string, string>): string {
  return `nos://sync?${new URLSearchParams(params).toString()}`;
}

const KEY = 'Ab'.repeat(32);

describe('parsePairingLink', () => {
  it('reads every field of the link the web interface builds', () => {
    const link = webLink({
      server: 'https://nas.local:8443/',
      token: 'nos_dt_abc',
      refresh: 'nos_rt_def',
      key: KEY,
      name: 'My Phone & tablet',
    });

    const p = parsePairingLink(link);

    expect(p).toEqual({
      serverUrl: 'https://nas.local:8443',
      deviceToken: 'nos_dt_abc',
      refreshToken: 'nos_rt_def',
      serverKey: KEY.toLowerCase(),
      deviceName: 'My Phone & tablet',
    });
  });

  it('matches the desktop client on its own test link', () => {
    const p = parsePairingLink(
      `nos://sync?server=https%3A%2F%2Fnas.local%2F&token=nos_dt_abc&refresh=nos_rt_def&key=${KEY}&name=Laptop`,
    );

    expect(p.serverUrl).toBe('https://nas.local');
    expect(p.serverKey).toBe(KEY.toLowerCase());
  });

  it('leaves out refresh, key and name when the link has none', () => {
    const p = parsePairingLink(
      '  nos://sync?server=http://10.0.0.5&token=nos_dt_x  ',
    );

    expect(p).toEqual({
      serverUrl: 'http://10.0.0.5',
      deviceToken: 'nos_dt_x',
      refreshToken: null,
      serverKey: null,
      deviceName: null,
    });
  });

  it('takes a key with colons and spaces', () => {
    const colons = (KEY.match(/../g) ?? []).join(':');

    const p = parsePairingLink(
      webLink({server: 'https://nas', token: 'nos_dt_x', key: colons}),
    );

    expect(p.serverKey).toBe(KEY.toLowerCase());
  });

  it.each([
    ['not nos://', 'https://nas.local'],
    ['another nos:// link', 'nos://discover?server=https://nas.local'],
    ['no server', 'nos://sync?token=nos_dt_abc'],
    ['not an http(s) server', 'nos://sync?server=ftp://x&token=nos_dt_abc'],
    ['a server without a host', 'nos://sync?server=https://&token=nos_dt_abc'],
    [
      'a server with a user name',
      'nos://sync?server=https://a:b@nas.local&token=nos_dt_abc',
    ],
    ['not a device token', 'nos://sync?server=https://nas.local&token=abc'],
    [
      'a short key',
      'nos://sync?server=https://nas.local&token=nos_dt_abc&key=short',
    ],
    [
      'a key that is not hex',
      `nos://sync?server=https://nas.local&token=nos_dt_abc&key=${'zz'.repeat(
        32,
      )}`,
    ],
    ['bad escapes', 'nos://sync?server=%E0%A4%A&token=nos_dt_abc'],
    ['', ''],
  ])('refuses %s', (_why, link) => {
    expect(() => parsePairingLink(link)).toThrow(PairingError);
  });
});

describe('normalizeKeyPin', () => {
  it('lowercases and strips separators', () => {
    expect(normalizeKeyPin(` ${KEY.toUpperCase()} `)).toBe(KEY.toLowerCase());
  });
});

describe('serverHost', () => {
  it.each([
    ['https://NAS.local', 'nas.local'],
    ['https://nas.local:8443/x', 'nas.local'],
    ['http://10.0.0.5:80', '10.0.0.5'],
    ['https://[fe80::1]:443', 'fe80::1'],
  ])('%s is kept under %s', (url, host) => {
    expect(serverHost(url)).toBe(host);
  });

  it.each([
    'ftp://nas',
    'https://',
    'nas.local',
    'https://user@nas.local',
    'https://evil.example\\@nas.local',
    'https://nas local',
    'https://nås.local',
  ])('%s has none', url => {
    expect(serverHost(url)).toBeNull();
  });
});
