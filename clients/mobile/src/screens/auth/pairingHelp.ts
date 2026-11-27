/** Where the web interface shows the pairing link and its QR code. */
export const PAIRING_STEPS =
  'In the nOS web interface, open Settings → Sync → Devices, choose Add Device, ' +
  'name it and pick Android. Scan the QR code it shows, or copy the pairing link.';

/**
 * A friendlier message for a code that isn't a pairing link. The web
 * interface's "Scan QR Code" button shows a nos://discover code, which only
 * names the server and can't pair a device.
 */
export function explainBadLink(link: string, reason: string): string {
  if (/^nos:\/\/discover/i.test(link.trim())) {
    return `That code only names the server; it can't pair this phone. ${PAIRING_STEPS}`;
  }
  return reason;
}
