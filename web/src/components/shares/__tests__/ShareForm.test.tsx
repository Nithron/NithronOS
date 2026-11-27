import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ShareForm } from '../ShareForm'
import type { Share } from '@/api/shares'

// Radix's switch measures itself; jsdom has no ResizeObserver.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver

vi.mock('@/api/shares', async (orig) => ({
  ...(await orig<typeof import('@/api/shares')>()),
  usePolicy: () => ({ data: undefined }),
}))
vi.mock('@/api/apps', () => ({ appsApi: { installed: vi.fn().mockResolvedValue([]) } }))

const existing: Share = {
  id: 's1',
  name: 'docs',
  path: '/srv/shares/docs',
  owners: ['user:admin'],
  sync: { enabled: false },
  created_at: '2026-11-27T00:00:00Z',
  updated_at: '2026-11-27T00:00:00Z',
}

function renderForm(share?: Share) {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter>
        <ShareForm share={share} mode={share ? 'edit' : 'create'} onSubmit={vi.fn()} onCancel={vi.fn()} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  fireEvent.click(screen.getByRole('tab', { name: 'Protocols' }))
  return screen.getByRole('switch', { name: 'SMB file sharing' })
}

describe('ShareForm SMB default', () => {
  it('shows SMB off for an existing share without SMB', () => {
    expect(renderForm(existing).getAttribute('aria-checked')).toBe('false')
  })

  it('offers SMB on for a new share', () => {
    expect(renderForm().getAttribute('aria-checked')).toBe('true')
  })
})
