/**
 * Dates and times in the reader's own locale (Intl), never a fixed US form:
 * the locale decides the order and its clock, 24-hour wherever the locale
 * uses one. A value that is not a date shows as "-".
 */
type DateInput = string | number | Date | null | undefined

const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const dateOnly = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const timeOnly = new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' })

function toDate(v: DateInput): Date | null {
  if (v === null || v === undefined || v === '') return null
  const d = v instanceof Date ? v : new Date(v)
  return Number.isNaN(d.getTime()) ? null : d
}

export function formatDateTime(v: DateInput): string {
  const d = toDate(v)
  return d ? dateTime.format(d) : '-'
}

export function formatDate(v: DateInput): string {
  const d = toDate(v)
  return d ? dateOnly.format(d) : '-'
}

/** A time with seconds (log lines, a deadline). */
export function formatTime(v: DateInput): string {
  const d = toDate(v)
  return d ? timeOnly.format(d) : '-'
}
