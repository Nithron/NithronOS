import { describe, expect, it } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { ConfirmProvider, confirmAction } from '../confirm'

describe('confirmAction', () => {
  it('resolves true when the action is chosen', async () => {
    render(<ConfirmProvider />)
    let answer: Promise<boolean> = Promise.resolve(false)
    act(() => {
      answer = confirmAction({ title: 'Delete the schedule nightly?', action: 'Delete schedule' })
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Delete schedule' }))
    await expect(answer).resolves.toBe(true)
  })

  it('resolves false on cancel', async () => {
    render(<ConfirmProvider />)
    let answer: Promise<boolean> = Promise.resolve(true)
    act(() => {
      answer = confirmAction({ title: 'Remove this peer?', action: 'Remove peer' })
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel' }))
    await expect(answer).resolves.toBe(false)
  })
})
