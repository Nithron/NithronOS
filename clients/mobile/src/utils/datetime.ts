/**
 * Dates and times in the reader's own locale (Intl), never a fixed US
 * form: the locale decides the order and its clock, 24-hour wherever the
 * locale uses one. Where Intl is missing, the form is 2026-11-27 22:40.
 * A value that is not a date shows as "-".
 */
type DateInput = string | number | Date | null | undefined;

const DATE_TIME: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
};
const DATE_ONLY: Intl.DateTimeFormatOptions = {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
};

function toDate(v: DateInput): Date | null {
  if (v === null || v === undefined || v === '') {
    return null;
  }
  const d = v instanceof Date ? v : new Date(v);
  return Number.isNaN(d.getTime()) ? null : d;
}

const two = (n: number) => String(n).padStart(2, '0');

function isoDate(d: Date): string {
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}`;
}

function format(d: Date, options: Intl.DateTimeFormatOptions): string {
  try {
    return new Intl.DateTimeFormat(undefined, options).format(d);
  } catch {
    const date = isoDate(d);
    return options.hour
      ? `${date} ${two(d.getHours())}:${two(d.getMinutes())}`
      : date;
  }
}

/** A date with its time, to the minute. */
export function formatDateTime(v: DateInput): string {
  const d = toDate(v);
  return d ? format(d, DATE_TIME) : '-';
}

/** A date alone. */
export function formatDate(v: DateInput): string {
  const d = toDate(v);
  return d ? format(d, DATE_ONLY) : '-';
}
