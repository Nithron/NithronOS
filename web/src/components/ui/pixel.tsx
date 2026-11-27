/**
 * The pixel layer of the identity that needs no drawn asset: marks, meters
 * and the working cursor, on whole pixels only (crisp edges, whole-number
 * sizes, no circles, no smooth bars). Icons, the pixel display face and
 * pictures come from the shared set later (components/icons.ts and the
 * pixel-display class in styles.css are their slots).
 */
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** The state of a thing, as a mark beside its word. */
export type MarkState = 'ok' | 'warning' | 'failed' | 'running' | 'notice' | 'off'

const markTone: Record<MarkState, string> = {
  ok: 'text-status-ok',
  warning: 'text-status-warning',
  failed: 'text-status-failed',
  running: 'text-ink',
  notice: 'text-ink',
  off: 'text-dim',
}

/** An 8 by 8 pixel cross, one pixel wide, corner to corner. */
const CROSS = Array.from({ length: 8 }, (_, i) => `M${i} ${i}h1v1h-1zM${7 - i} ${i}h1v1h-1z`).join(
  '',
)
/** An 8 by 8 square with a one-pixel edge. */
const OPEN = 'M0 0h8v8h-8zM1 1v6h6v-6z'

interface MarkProps {
  state: MarkState
  className?: string
}

/**
 * Filled square: in order. Open square: warning (and off, in grey). Cross:
 * failed. A square that blinks: running. A small ink square: a notice,
 * which is no state. Never alone: put its word beside it (StatusMark does).
 */
export function Mark({ state, className }: MarkProps) {
  const shape =
    state === 'failed' ? (
      <path d={CROSS} />
    ) : state === 'warning' || state === 'off' ? (
      <path d={OPEN} fillRule="evenodd" />
    ) : state === 'notice' ? (
      <rect x="2" y="2" width="4" height="4" />
    ) : (
      <rect width="8" height="8" />
    )
  return (
    <svg
      viewBox="0 0 8 8"
      width="8"
      height="8"
      shapeRendering="crispEdges"
      aria-hidden
      className={cn(
        'inline-block shrink-0 fill-current',
        markTone[state],
        state === 'running' && 'pixel-blink',
        className,
      )}
    >
      {shape}
    </svg>
  )
}

interface StatusMarkProps extends MarkProps {
  children: ReactNode
}

/** A mark and the word that says the state. */
export function StatusMark({ state, children, className }: StatusMarkProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <Mark state={state} />
      <span>{children}</span>
    </span>
  )
}

interface MeterProps {
  value: number
  max?: number
  /** How many blocks the full measure has. */
  blocks?: number
  /** progress: a task on its way; meter: a measure such as disk use. */
  kind?: 'progress' | 'meter'
  tone?: 'ink' | 'ok' | 'warning' | 'failed'
  label: string
  /** The number beside the blocks; defaults to the percentage. */
  valueText?: string
  showValue?: boolean
  className?: string
}

const blockTone = {
  ink: 'bg-ink',
  ok: 'bg-status-ok',
  warning: 'bg-status-warning',
  failed: 'bg-status-failed',
} as const

/**
 * A row of square blocks that fill one by one, with its number as text.
 * A block fills only when the measure covers it whole.
 */
export function Meter({
  value,
  max = 100,
  blocks = 20,
  kind = 'meter',
  tone = 'ink',
  label,
  valueText,
  showValue = true,
  className,
}: MeterProps) {
  const ratio = max > 0 ? Math.min(Math.max(value / max, 0), 1) : 0
  const filled = Math.floor(ratio * blocks)
  const text = valueText ?? `${Math.round(ratio * 100)}%`
  return (
    <span className={cn('inline-flex items-center gap-3', className)}>
      <span
        role={kind === 'progress' ? 'progressbar' : 'meter'}
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-valuetext={text}
        className="inline-flex gap-[2px]"
      >
        {Array.from({ length: blocks }, (_, i) => (
          <span
            key={i}
            className={cn('h-2 w-2 shrink-0', i < filled ? blockTone[tone] : 'border border-rule')}
          />
        ))}
      </span>
      {showValue && <span className="font-mono text-sm tabular-nums">{text}</span>}
    </span>
  )
}

interface WorkingProps {
  children?: ReactNode
  className?: string
}

/** Loading or working: words in the mono face and a block cursor. */
export function Working({ children = 'Loading', className }: WorkingProps) {
  return (
    <span role="status" className={cn('font-mono text-sm text-ink-2', className)}>
      <span className="pixel-cursor">{children}</span>
    </span>
  )
}
