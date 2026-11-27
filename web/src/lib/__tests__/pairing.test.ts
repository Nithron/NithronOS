import { describe, expect, it } from 'vitest'
import { pairingLink } from '../pairing'

describe('pairingLink', () => {
  it('carries the server, both tokens, the key and the name', () => {
    const link = pairingLink({
      server: 'https://nas.local',
      deviceToken: 'nos_dt_a+b=',
      refreshToken: 'nos_rt_c',
      serverKey: 'ab'.repeat(32),
      deviceName: 'My Laptop',
    })
    expect(link.startsWith('nos://sync?')).toBe(true)
    const q = new URLSearchParams(link.slice('nos://sync?'.length))
    expect(q.get('server')).toBe('https://nas.local')
    expect(q.get('token')).toBe('nos_dt_a+b=')
    expect(q.get('refresh')).toBe('nos_rt_c')
    expect(q.get('key')).toBe('ab'.repeat(32))
    expect(q.get('name')).toBe('My Laptop')
  })

  it('leaves out what it does not have', () => {
    const q = new URLSearchParams(pairingLink({ server: 'https://x', deviceToken: 'nos_dt_1' }).split('?')[1])
    expect([...q.keys()]).toEqual(['server', 'token'])
  })
})
