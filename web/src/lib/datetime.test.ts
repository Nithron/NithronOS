import { describe, expect, it } from 'vitest'
import { formatDate, formatDateTime, formatTime } from './datetime'

describe('datetime', () => {
  const iso = '2026-11-27T21:05:09Z'

  it('formats in the reader locale, not a fixed form', () => {
    expect(formatDateTime(iso)).toBe(
      new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(iso)),
    )
    expect(formatDate(iso)).toBe(new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(new Date(iso)))
    expect(formatTime(iso)).toBe(new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' }).format(new Date(iso)))
  })

  it('shows a dash for no date or a broken one', () => {
    expect(formatDateTime(undefined)).toBe('-')
    expect(formatDate('')).toBe('-')
    expect(formatTime('not a date')).toBe('-')
  })
})
