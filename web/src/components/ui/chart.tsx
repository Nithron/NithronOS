/**
 * Charts on the identity (recharts props): hairline axes and grid, tick
 * labels in the mono face in --dim, series in --series-1..5 as 1.5 px lines
 * with no fill under them, told apart by their line style as well as their
 * colour. Every series has its name and latest value as text (SeriesKey).
 */
export type SeriesNumber = 1 | 2 | 3 | 4 | 5

const dashes: Record<SeriesNumber, string | undefined> = {
  1: undefined,
  2: '4 3',
  3: '1 3',
  4: '8 3 1 3',
  5: '2 2',
}

export const chartAxis = {
  stroke: 'var(--hairline)',
  tickLine: false,
  tick: { fill: 'var(--dim)', fontSize: 13, fontFamily: 'var(--font-mono)' },
} as const

/** Room at the right for the last time label, so no text is clipped. */
export const chartMargin = { top: 8, right: 40, bottom: 0, left: 8 } as const

export const chartGrid = { stroke: 'var(--hairline)', vertical: false } as const

export const chartTooltip = {
  contentStyle: {
    background: 'var(--page)',
    border: '1px solid var(--edge)',
    borderRadius: 0,
    fontFamily: 'var(--font-mono)',
    fontSize: 13,
  },
  labelStyle: { color: 'var(--ink-2)' },
  itemStyle: { color: 'var(--ink)' },
  cursor: { stroke: 'var(--rule)' },
} as const

/** The props of a series' line: its colour, 1.5 px, its dash, no dots. */
export function seriesLine(n: SeriesNumber) {
  return {
    stroke: `var(--series-${n})`,
    strokeWidth: 1.5,
    strokeDasharray: dashes[n],
    dot: false,
    isAnimationActive: false,
    type: 'monotone' as const,
  }
}

export interface SeriesKeyItem {
  n: SeriesNumber
  name: string
  /** The latest value, as text. */
  value: string
}

/** Each series: a short piece of its line, its name and its latest value. */
export function SeriesKey({ items }: { items: SeriesKeyItem[] }) {
  return (
    <ul className="mt-4 flex flex-wrap gap-x-8 gap-y-2 border-t border-hairline pt-3 text-sm">
      {items.map((s) => (
        <li key={s.name} className="flex items-center gap-2">
          <svg width="24" height="8" aria-hidden className="shrink-0">
            <line x1="0" y1="4" x2="24" y2="4" stroke={`var(--series-${s.n})`} strokeWidth="1.5" strokeDasharray={dashes[s.n]} />
          </svg>
          <span>{s.name}</span>
          <span className="font-mono tabular-nums">{s.value}</span>
        </li>
      ))}
    </ul>
  )
}
