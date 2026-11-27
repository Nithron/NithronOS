/**
 * Where the pairing is kept: the device and refresh tokens are the
 * device's credentials, so they live in the platform's secure store
 * (Android Keystore, iOS Keychain) through react-native-keychain, not in
 * AsyncStorage.
 */

import * as Keychain from 'react-native-keychain';

export interface Credentials {
  serverUrl: string;
  deviceToken: string;
  refreshToken: string | null;
  /** The pinned server key (SHA-256 of its TLS public key, hex) */
  serverKey: string | null;
}

const SERVICE = 'com.nithron.sync.pairing';
const ACCOUNT = 'nos-device';

export async function loadCredentials(): Promise<Credentials | null> {
  const entry = await Keychain.getGenericPassword({service: SERVICE});
  if (!entry) {
    return null;
  }
  return decodeCredentials(entry.password);
}

export async function saveCredentials(credentials: Credentials): Promise<void> {
  const saved = await Keychain.setGenericPassword(
    ACCOUNT,
    JSON.stringify(credentials),
    {
      service: SERVICE,
      accessible: Keychain.ACCESSIBLE.AFTER_FIRST_UNLOCK_THIS_DEVICE_ONLY,
    },
  );
  if (!saved) {
    throw new Error("The pairing couldn't be saved on this device");
  }
}

export async function clearCredentials(): Promise<void> {
  await Keychain.resetGenericPassword({service: SERVICE});
}

/** null when what's stored isn't a pairing (an older or damaged entry). */
export function decodeCredentials(raw: string): Credentials | null {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null) {
    return null;
  }
  const v = value as Record<string, unknown>;
  if (typeof v.serverUrl !== 'string' || typeof v.deviceToken !== 'string') {
    return null;
  }
  return {
    serverUrl: v.serverUrl,
    deviceToken: v.deviceToken,
    refreshToken: typeof v.refreshToken === 'string' ? v.refreshToken : null,
    serverKey: typeof v.serverKey === 'string' ? v.serverKey : null,
  };
}
