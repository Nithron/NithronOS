/**
 * nOS's web certificate is self-signed, so the app trusts the server's key
 * from the pairing link instead of certificate authorities. On Android the
 * check is in the app's HTTP client (android/.../pinning); this tells it
 * which key a host must present. The native side keeps the pins itself, so
 * they apply from the first request after a restart.
 */

import {NativeModules, Platform} from 'react-native';

interface NosPinningModule {
  setPin(host: string, pinHex: string): Promise<void>;
  clearPin(host: string): Promise<void>;
  clearAll(): Promise<void>;
}

const nativePinning: NosPinningModule | undefined = NativeModules.NosPinning;

/** Whether this build can trust a pinned key (Android only, for now). */
export function pinningAvailable(): boolean {
  return nativePinning !== undefined;
}

export async function setServerPin(
  host: string,
  pinHex: string,
): Promise<void> {
  if (!nativePinning) {
    throw new Error(
      `Trusting a nOS server's own certificate isn't supported on ${Platform.OS} yet`,
    );
  }
  await nativePinning.setPin(host, pinHex);
}

export async function clearServerPins(): Promise<void> {
  if (nativePinning) {
    await nativePinning.clearAll();
  }
}
