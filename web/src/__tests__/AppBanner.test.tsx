import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from '../App'

describe('Global banner on proxy misconfig', () => {
  it('shows banner when /api/v1/setup/state returns HTML', async () => {
    // HTML response for setup state
    global.fetch = vi.fn().mockImplementation((url: string) => {
      if (url === '/api/v1/setup/state') {
        return Promise.resolve({
          ok: true,
          status: 200,
          headers: { get: () => 'text/html' },
          text: async () => '<!doctype html>nope',
        })
      }
      // default other calls
      return Promise.resolve({ ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({}) })
    })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <App />
      </QueryClientProvider>
    )
    const el = await screen.findByText(/Backend unreachable or proxy misconfigured/i)
    expect(!!el).toBe(true)
    const link = await screen.findByRole('link', { name: /Troubleshooting/i })
    expect(link.getAttribute('href')).toBe('/help/proxy')
  })
})


