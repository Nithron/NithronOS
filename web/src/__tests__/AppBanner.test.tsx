import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from '../App'
import http, { ProxyMisconfiguredError } from '@/lib/nos-client'

describe('Global banner on proxy misconfig', () => {
  it('shows banner when /api/v1/setup/state returns HTML', async () => {
    // The proxy serves HTML at /api: nos-client reports it as such.
    vi.spyOn(http.setup, 'getState').mockRejectedValue(new ProxyMisconfiguredError('HTML at /api'))

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


