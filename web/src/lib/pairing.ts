/**
 * The pairing link a NithronSync client takes (desktop "Pair", daemon
 * --pair, the mobile app's QR scan); clients/sync-core/api/pairing.go
 * parses it.
 */
export interface PairingInfo {
  server: string
  deviceToken: string
  refreshToken?: string
  /** SHA-256 of the server's TLS key, pinned by the client */
  serverKey?: string
  deviceName?: string
}

export function pairingLink(p: PairingInfo): string {
  const q = new URLSearchParams({ server: p.server, token: p.deviceToken })
  if (p.refreshToken) q.set('refresh', p.refreshToken)
  if (p.serverKey) q.set('key', p.serverKey)
  if (p.deviceName) q.set('name', p.deviceName)
  return `nos://sync?${q.toString()}`
}
