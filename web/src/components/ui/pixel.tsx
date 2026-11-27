/**
 * The pixel layer: state marks (the set's six state icons), meters as rows
 * of square blocks with their number in the pixel face, and loading as
 * words in the pixel face with a block cursor. On whole pixels only.
 */
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { FailedIcon, NoticeIcon, OfflineIcon, OkIcon, WarningIcon, WorkingIcon, type IconComponent } from '@/components/icons'

/** The state of a thing, as a mark beside its word. */
export type MarkState = 'ok' | 'warning' | 'failed' | 'running' | 'notice' | 'off'

const markTone: Record<MarkState, string> = {
  ok: 'text-status-ok',
  warning: 'text-status-warning',
  failed: 'text-status-failed',
  running: 'text-ink',
  notice: 'text-ink',
  off: 'text-ink-2',
}

const markIcon: Record<MarkState, IconComponent> = {
  ok: OkIcon,
  warning: WarningIcon,
  failed: FailedIcon,
  running: WorkingIcon,
  notice: NoticeIcon,
  off: OfflineIcon,
}

interface MarkProps {
  state: MarkState
  className?: string
}

/**
 * The set's six state icons at 16 px: ok, warning, failed, running (it
 * blinks), notice (no state), off. Never alone: put its word beside it
 * (StatusMark does).
 */
export function Mark({ state, className }: MarkProps) {
  const Icon = markIcon[state]
  return <Icon className={cn(markTone[state], className)} />
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
      {showValue && <span className="pixel-display">{text}</span>}
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
    <span role="status" className={cn('pixel-display text-ink-2', className)}>
      <span className="pixel-cursor">{children}</span>
    </span>
  )
}
