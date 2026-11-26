import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'

// Hoist http client mock before importing hooks
vi.mock('../lib/nos-client', () => {
  const get = vi.fn()
  const post = vi.fn()
  return { default: { get, post } }
})

import http from '../lib/nos-client'
import { 
  useSystemInfo, 
  usePools, 
  useSmartSummary, 
  useShares,
  useInstalledApps,
} from '../hooks/use-api'

// Create a wrapper with QueryClient
const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { 
        retry: false,
        gcTime: 0,
        staleTime: 0,
      },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  )
}

describe('API Wiring Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('System API', () => {
    it('should fetch system info using /v1/system/info', async () => {
      const mockData = {
        hostname: 'nos',
        uptime: 123456,
        kernel: '5.15.0',
        version: '0.3.0',
      }
      vi.mocked(http.get).mockResolvedValueOnce(mockData as any)
      const { result } = renderHook(() => useSystemInfo(), { wrapper: createWrapper() })
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(http.get).toHaveBeenCalledWith('/v1/system/info')
      expect(result.current.data).toEqual(mockData)
    })
  })

  describe('Storage API', () => {
    it('should fetch pools using /v1/pools', async () => {
      const mockPools = [
        {
          id: 'pool1',
          uuid: 'uuid1',
          label: 'Main Pool',
          mountpoint: '/mnt/main',
          size: 1000000,
          used: 500000,
          free: 500000,
          raid: 'raid1',
          status: 'online',
          devices: [],
        },
      ]
      vi.mocked(http.get).mockResolvedValueOnce(mockPools as any)
      const { result } = renderHook(() => usePools(), { wrapper: createWrapper() })
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(http.get).toHaveBeenCalledWith('/v1/pools')
      expect(result.current.data).toEqual(mockPools)
    })
  })

  describe('Health API', () => {
    it('should fetch SMART summary using /v1/smart/summary', async () => {
      const mockSummary = {
        totalDevices: 4,
        healthyDevices: 3,
        warningDevices: 1,
        criticalDevices: 0,
      }
      vi.mocked(http.get).mockResolvedValueOnce(mockSummary as any)
      const { result } = renderHook(() => useSmartSummary(), { wrapper: createWrapper() })
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(http.get).toHaveBeenCalledWith('/v1/smart/summary')
      expect(result.current.data).toEqual(mockSummary)
    })
  })

  describe('Shares API', () => {
    it('should fetch shares using /v1/shares', async () => {
      const mockShares = [
        { name: 'Documents', path: '/mnt/main/docs', protocol: 'smb', enabled: true },
      ]
      vi.mocked(http.get).mockResolvedValueOnce(mockShares as any)
      const { result } = renderHook(() => useShares(), { wrapper: createWrapper() })
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(http.get).toHaveBeenCalledWith('/v1/shares')
      expect(result.current.data).toEqual(mockShares)
    })
  })

  describe('Apps API', () => {
    it('should fetch installed apps using /v1/apps/installed', async () => {
      const mockApps = { items: [ { id: 'nextcloud', name: 'Nextcloud', version: '28.0', status: 'running' } ] }
      vi.mocked(http.get).mockResolvedValueOnce(mockApps as any)
      const { result } = renderHook(() => useInstalledApps(), { wrapper: createWrapper() })
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(http.get).toHaveBeenCalledWith('/v1/apps/installed')
      expect(result.current.data).toEqual(mockApps.items)
    })
  })
})
