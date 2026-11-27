import {
  davPath,
  joinPath,
  parseFolderListing,
  parseHttpDate,
} from '../src/api/webdav';

// What golang.org/x/net/webdav (nosd's WebDAV) answers to PROPFIND Depth 1
// on /dav/share1/docs/ (recorded, lock entries trimmed).
const LISTING =
  '<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:">' +
  '<D:response><D:href>/dav/share1/docs/</D:href><D:propstat><D:prop><D:resourcetype><D:collection xmlns:D="DAV:"/></D:resourcetype><D:displayname>docs</D:displayname><D:getlastmodified>Fri, 27 Nov 2026 10:58:00 GMT</D:getlastmodified></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>' +
  '<D:response><D:href>/dav/share1/docs/a%20b&amp;c.txt</D:href><D:propstat><D:prop><D:resourcetype></D:resourcetype><D:displayname>a b&amp;c.txt</D:displayname><D:getcontentlength>6</D:getcontentlength><D:getlastmodified>Fri, 27 Nov 2026 10:58:00 GMT</D:getlastmodified><D:getcontenttype>text/plain; charset=utf-8</D:getcontenttype><D:getetag>"18ebe1bd7c0ae19b6"</D:getetag></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>' +
  '<D:response><D:href>/dav/share1/docs/Sub%20Folder/</D:href><D:propstat><D:prop><D:resourcetype><D:collection xmlns:D="DAV:"/></D:resourcetype><D:displayname>Sub Folder</D:displayname><D:getlastmodified>Fri, 27 Nov 2026 10:58:00 GMT</D:getlastmodified></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>' +
  '</D:multistatus>';

describe('parseFolderListing', () => {
  it("lists a folder's entries without the folder, folders first", () => {
    const entries = parseFolderListing(LISTING, 'share1', 'docs');

    expect(entries).toEqual([
      {
        path: 'docs/Sub Folder',
        name: 'Sub Folder',
        isDir: true,
        size: 0,
        modified: '2026-11-27T10:58:00.000Z',
        contentType: null,
      },
      {
        path: 'docs/a b&c.txt',
        name: 'a b&c.txt',
        isDir: false,
        size: 6,
        modified: '2026-11-27T10:58:00.000Z',
        contentType: 'text/plain; charset=utf-8',
      },
    ]);
  });

  it('reads other prefixes and absolute hrefs', () => {
    const xml =
      '<d:multistatus xmlns:d="DAV:"><d:response><d:href>https://nas/dav/s/x.bin</d:href>' +
      '<d:propstat><d:prop><d:getcontentlength>42</d:getcontentlength></d:prop></d:propstat></d:response></d:multistatus>';

    const [entry] = parseFolderListing(xml, 's', '');

    expect(entry).toMatchObject({path: 'x.bin', name: 'x.bin', size: 42});
  });

  it("skips hrefs outside the share and the share's root", () => {
    const xml =
      '<D:multistatus><D:response><D:href>/dav/other/x</D:href></D:response>' +
      '<D:response><D:href>/dav/s/</D:href></D:response></D:multistatus>';

    expect(parseFolderListing(xml, 's', '/')).toEqual([]);
  });
});

describe('davPath', () => {
  it('escapes each segment and marks folders', () => {
    expect(davPath('share1', 'docs/a b&c.txt', false)).toBe(
      '/dav/share1/docs/a%20b%26c.txt',
    );
    expect(davPath('share1', '/docs/', true)).toBe('/dav/share1/docs/');
    expect(davPath('share1', '', true)).toBe('/dav/share1/');
  });

  it("can't leave the share", () => {
    expect(davPath('s', '../../etc/passwd', false)).toBe('/dav/s/etc/passwd');
  });
});

describe('joinPath', () => {
  it('joins from the root', () => {
    expect(joinPath('', 'a')).toBe('a');
    expect(joinPath('a/', 'b')).toBe('a/b');
  });
});

describe('parseHttpDate', () => {
  it('reads RFC 1123 dates and refuses others', () => {
    expect(parseHttpDate('Mon, 02 Jan 2006 15:04:05 GMT')).toBe(
      '2006-01-02T15:04:05.000Z',
    );
    expect(parseHttpDate('yesterday')).toBeNull();
    expect(parseHttpDate(null)).toBeNull();
  });
});
