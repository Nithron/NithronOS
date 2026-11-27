import {ApiError, NosClient, SessionEndedError} from '../src/api/client';
import {loadCredentials, saveCredentials} from '../src/api/credentials';
import {parsePairingLink} from '../src/api/pairing';
import {clearServerPins, setServerPin} from '../src/native/pinning';

jest.mock('../src/native/pinning', () => ({
  setServerPin: jest.fn(async () => undefined),
  clearServerPins: jest.fn(async () => undefined),
}));

interface Call {
  url: string;
  method: string;
  auth: string | undefined;
  body: string | undefined;
}

type Reply = {status: number; body?: unknown};
type Route = (call: Call) => Reply;

function fakeResponse({status, body}: Reply): Response {
  const text = body === undefined ? '' : JSON.stringify(body);
  return {
    status,
    ok: status >= 200 && status < 300,
    text: async () => text,
    json: async () => JSON.parse(text),
  } as unknown as Response;
}

/** A fake nosd: route answers each request; calls records them. */
function fakeServer(route: Route) {
  const calls: Call[] = [];
  const fetchFn = jest.fn(
    async (url: RequestInfo | URL, init?: RequestInit) => {
      const headers = (init?.headers ?? {}) as Record<string, string>;
      const call: Call = {
        url: String(url),
        method: init?.method ?? 'GET',
        auth: headers.Authorization,
        body: init?.body as string | undefined,
      };
      calls.push(call);
      return fakeResponse(route(call));
    },
  );
  return {calls, client: new NosClient(fetchFn as unknown as typeof fetch)};
}

const KEY = 'ab'.repeat(32);
const LINK =
  `nos://sync?server=https%3A%2F%2Fnas.local%3A8443&token=nos_dt_old` +
  `&refresh=nos_rt_old&key=${KEY}`;
const CONFIG = {
  device_id: 'dev-1',
  sync_shares: [],
  selective_paths: null,
  bandwidth_limit_kbps: 0,
  pause_sync: false,
  sync_on_mobile_data: false,
};
const NEW_TOKENS = {
  device_token: 'nos_dt_new',
  refresh_token: 'nos_rt_new',
  expires_at: '2027-02-25T10:00:00Z',
};

/** A client paired with a server that then answers with route. */
async function paired(route: Route) {
  let pairing = true;
  const server = fakeServer(call =>
    pairing ? {status: 200, body: CONFIG} : route(call),
  );
  await server.client.pair(parsePairingLink(LINK));
  pairing = false;
  server.calls.length = 0;
  return server;
}

/** Valid token: nos_dt_new only (the old one has expired). */
function refreshingServer(extra?: (call: Call) => Reply | undefined): Route {
  return call => {
    if (call.url.endsWith('/api/v1/sync/devices/refresh')) {
      return {status: 200, body: NEW_TOKENS};
    }
    const reply = extra?.(call);
    if (reply) {
      return reply;
    }
    if (call.auth !== 'Bearer nos_dt_new') {
      return {
        status: 401,
        body: {
          error: {code: 'auth.invalid', message: 'Invalid or expired token'},
        },
      };
    }
    return {
      status: 200,
      body: {shares: [{share_id: 's1', share_name: 'Docs'}], count: 1},
    };
  };
}

beforeEach(async () => {
  jest.clearAllMocks();
  const {clearCredentials} = jest.requireActual('../src/api/credentials');
  await clearCredentials();
});

describe('pairing', () => {
  it('pins the key, checks the token, then keeps the pairing', async () => {
    const {client, calls} = fakeServer(() => ({status: 200, body: CONFIG}));

    const config = await client.pair(parsePairingLink(LINK));

    expect(config.device_id).toBe('dev-1');
    expect(setServerPin).toHaveBeenCalledWith('nas.local', KEY);
    expect(calls).toEqual([
      expect.objectContaining({
        url: 'https://nas.local:8443/api/v1/sync/config',
        auth: 'Bearer nos_dt_old',
      }),
    ]);
    expect(await loadCredentials()).toEqual({
      serverUrl: 'https://nas.local:8443',
      deviceToken: 'nos_dt_old',
      refreshToken: 'nos_rt_old',
      serverKey: KEY,
    });
  });

  it("keeps nothing when the server doesn't take the token", async () => {
    const {client} = fakeServer(call =>
      call.url.endsWith('/refresh')
        ? {
            status: 401,
            body: {
              error: {code: 'auth.invalid', message: 'Invalid refresh token'},
            },
          }
        : {
            status: 401,
            body: {
              error: {
                code: 'auth.invalid',
                message: 'Invalid or expired token',
              },
            },
          },
    );

    await expect(client.pair(parsePairingLink(LINK))).rejects.toBeInstanceOf(
      SessionEndedError,
    );
    expect(client.isPaired).toBe(false);
    expect(await loadCredentials()).toBeNull();
  });

  it('restores a saved pairing and pins its key again', async () => {
    await saveCredentials({
      serverUrl: 'https://10.0.0.5',
      deviceToken: 'nos_dt_x',
      refreshToken: null,
      serverKey: KEY,
    });
    const {client} = fakeServer(() => ({status: 200}));

    expect(await client.restore()).toBe(true);
    expect(client.serverUrl).toBe('https://10.0.0.5');
    expect(setServerPin).toHaveBeenCalledWith('10.0.0.5', KEY);
  });

  it('forgets the pairing and the pins', async () => {
    const {client} = await paired(() => ({status: 200, body: CONFIG}));

    await client.forget();

    expect(client.isPaired).toBe(false);
    expect(clearServerPins).toHaveBeenCalled();
    expect(await loadCredentials()).toBeNull();
  });
});

describe('expired device token', () => {
  it('refreshes both tokens and sends the request once more', async () => {
    const {client, calls} = await paired(refreshingServer());

    const shares = await client.listShares();

    expect(shares).toEqual([{share_id: 's1', share_name: 'Docs'}]);
    expect(
      calls.map(c => [c.url.replace('https://nas.local:8443', ''), c.auth]),
    ).toEqual([
      ['/api/v1/sync/shares', 'Bearer nos_dt_old'],
      ['/api/v1/sync/devices/refresh', undefined],
      ['/api/v1/sync/shares', 'Bearer nos_dt_new'],
    ]);
    expect(JSON.parse(calls[1].body ?? '')).toEqual({
      refresh_token: 'nos_rt_old',
    });
    expect(await loadCredentials()).toMatchObject({
      deviceToken: 'nos_dt_new',
      refreshToken: 'nos_rt_new',
    });
  });

  it('gives up after one retry instead of refreshing forever', async () => {
    const {client, calls} = await paired(
      refreshingServer(call =>
        call.url.endsWith('/shares')
          ? {
              status: 401,
              body: {
                error: {
                  code: 'auth.invalid',
                  message: 'Invalid or expired token',
                },
              },
            }
          : undefined,
      ),
    );

    const failure = client.listShares();

    await expect(failure).rejects.toMatchObject({
      status: 401,
      code: 'auth.invalid',
    });
    expect(calls).toHaveLength(3);
  });

  it('shares one refresh between requests that fail together', async () => {
    const {client, calls} = await paired(refreshingServer());

    await Promise.all([
      client.listShares(),
      client.listShares(),
      client.listShares(),
    ]);

    expect(calls.filter(c => c.url.endsWith('/refresh'))).toHaveLength(1);
  });

  it('ends the pairing when the refresh token is refused', async () => {
    const {client} = await paired(call =>
      call.url.endsWith('/refresh')
        ? {
            status: 401,
            body: {
              error: {code: 'auth.invalid', message: 'Invalid refresh token'},
            },
          }
        : {status: 401},
    );
    const ended = jest.fn();
    client.onSessionEnded = ended;

    await expect(client.listShares()).rejects.toBeInstanceOf(SessionEndedError);
    expect(ended).toHaveBeenCalledTimes(1);
    expect(client.isPaired).toBe(false);
    expect(await loadCredentials()).toBeNull();
  });

  it('keeps the pairing when the refresh fails for another reason', async () => {
    const {client} = await paired(call =>
      call.url.endsWith('/refresh') ? {status: 502} : {status: 401},
    );

    await expect(client.listShares()).rejects.toMatchObject({status: 502});
    expect(client.isPaired).toBe(true);
  });
});

describe('answers', () => {
  it("reads nosd's error body", async () => {
    const {client} = await paired(() => ({
      status: 404,
      body: {error: {code: 'share.not_found', message: 'Share not found'}},
    }));

    const failure = client.getFileMetadata('s1', 'a.txt');

    await expect(failure).rejects.toBeInstanceOf(ApiError);
    await expect(failure).rejects.toMatchObject({
      status: 404,
      code: 'share.not_found',
      message: 'Share not found',
    });
  });

  it('asks for metadata with the path escaped', async () => {
    const {client, calls} = await paired(() => ({
      status: 200,
      body: {path: 'a b.txt'},
    }));

    await client.getFileMetadata('s 1', 'docs/a b.txt');

    expect(calls[0].url).toBe(
      'https://nas.local:8443/api/v1/sync/files/s%201/metadata?path=docs%2Fa%20b.txt',
    );
  });

  it('renames with PATCH {device_name}', async () => {
    const {client, calls} = await paired(() => ({
      status: 200,
      body: {id: 'dev-1'},
    }));

    await client.renameDevice('dev-1', 'Phone');

    expect(calls[0]).toMatchObject({
      method: 'PATCH',
      body: '{"device_name":"Phone"}',
    });
  });

  it('lists a folder with PROPFIND', async () => {
    const {client, calls} = await paired(() => ({status: 207}));

    await client.listFolder('s1', 'docs');

    expect(calls[0]).toMatchObject({
      method: 'PROPFIND',
      url: 'https://nas.local:8443/dav/s1/docs/',
      auth: 'Bearer nos_dt_old',
    });
  });

  it('turns an empty share list into []', async () => {
    const {client} = await paired(() => ({
      status: 200,
      body: {shares: null, count: 0},
    }));

    expect(await client.listShares()).toEqual([]);
  });
});
