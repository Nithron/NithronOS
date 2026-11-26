import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/lib/nos-client', () => ({ default: { get: vi.fn(), post: vi.fn() } }))

import http from '@/lib/nos-client'
import { storageApi } from '../storage'

describe('storageApi', () => {
	beforeEach(() => {
		vi.mocked(http.get).mockReset()
		vi.mocked(http.post).mockReset()
	})

	it('unwraps the device list', async () => {
		vi.mocked(http.get).mockResolvedValue({ devices: [{ name: 'sda', path: '/dev/sda' }] })
		expect(await storageApi.devices()).toEqual([{ name: 'sda', path: '/dev/sda' }])
	})

	it('starts a scrub of one pool, not all of them', async () => {
		vi.mocked(http.post).mockResolvedValue([])
		await storageApi.startScrub('pool1')
		expect(http.post).toHaveBeenCalledWith('/v1/scrub/start', { poolId: 'pool1' })
	})

	it('asks for SMART by kernel name', async () => {
		vi.mocked(http.get).mockResolvedValue({})
		await storageApi.smartDevice('nvme0n1')
		expect(http.get).toHaveBeenCalledWith('/v1/smart/device/nvme0n1')
	})
})
