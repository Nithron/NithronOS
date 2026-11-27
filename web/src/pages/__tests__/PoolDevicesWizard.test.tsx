import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import http from '@/lib/nos-client'
import { PoolDetails } from '../PoolDetails'

vi.mock('@/lib/nos-client', () => {
  const pools = {
    list: vi.fn(),
    get: vi.fn(),
    planDevice: vi.fn(),
    applyDevice: vi.fn(),
    getMountOptions: vi.fn().mockResolvedValue({ mountOptions: 'compress=zstd:3,noatime' }),
    txLog: vi.fn().mockResolvedValue({ lines: [], nextCursor: 0 }),
  }
  const defaultExport = { pools }
  return { default: defaultExport, api: defaultExport }
})


describe('Devices wizard warnings and confirm gating', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Mock the pool data
    vi.mocked(http.pools.list).mockResolvedValue([{ id:'p1', mount:'/mnt/p1', uuid:'U', raid:'raid1', size:1, used:0, free:1, devices:[] }] as any)
    vi.mocked(http.pools.get).mockResolvedValue({
      id:'p1', 
      mount:'/mnt/p1', 
      uuid:'U', 
      raid:'raid1', 
      size:1, 
      used:0, 
      free:1, 
      devices:[]
    } as any)
  })

  it('shows plan warnings and blocks Apply until confirm matches', async () => {
    vi.mocked(http.pools.planDevice).mockResolvedValue({
      planId: 'x',
      steps: [{ id:'s1', description:'add devices', command:"btrfs device add /dev/sdb /mnt/p1" }],
      warnings: ['Pool is >80% full; balance may take longer.'],
      requiresBalance: true,
    } as any)
    vi.mocked(http.pools.applyDevice).mockResolvedValue({ ok:true, tx_id:'t1' } as any)

    render(
      <MemoryRouter initialEntries={["/pools/p1"]}>
        <Routes>
          <Route path="/pools/:id" element={<PoolDetails />} />
        </Routes>
      </MemoryRouter>
    )

    await screen.findByRole('heading', { name: /Pool p1/i })

    fireEvent.click(screen.getByRole('tab', { name: 'Devices' }))
    fireEvent.click(await screen.findByRole('radio', { name: 'add' }))

    const input = await screen.findByLabelText(/Devices to add/i)
    fireEvent.change(input, { target: { value: '/dev/sdb' } })

    fireEvent.click(screen.getByRole('button', { name: 'Show the plan' }))

    await screen.findByText(/Pool is >80% full/i)
    const applyBtn = screen.getByRole('button', { name: 'Apply the plan' }) as HTMLButtonElement
    expect(applyBtn.disabled).toBe(true)

    fireEvent.change(screen.getByLabelText(/to apply the plan/i), { target: { value: 'ADD' } })
    await waitFor(() => expect(applyBtn.disabled).toBe(false))

    fireEvent.click(applyBtn)
    await waitFor(() => expect(http.pools.applyDevice).toHaveBeenCalled())
  })
})


