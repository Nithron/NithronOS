import * as React from 'react'
import { cn } from '@/lib/utils'
import { Meter } from '@/components/ui/pixel'

interface ProgressProps extends Omit<React.HTMLAttributes<HTMLSpanElement>, 'children'> {
  value?: number
  max?: number
  /** What is measured, for a screen reader. */
  label?: string
  /** A measure near its limit is a state: warning or failed. */
  tone?: 'ink' | 'ok' | 'warning' | 'failed'
  /** Before the rework this coloured the bar; use tone. */
  indicatorClassName?: string
}

/**
 * A measure as a row of square blocks (components/ui/pixel.tsx). The caller
 * shows the number beside it as text.
 */
const Progress = React.forwardRef<HTMLSpanElement, ProgressProps>(
  ({ className, value = 0, max = 100, label = 'Progress', tone = 'ink', indicatorClassName: _indicator, ...props }, ref) => (
    <span ref={ref} className={cn('flex', className)} {...props}>
      <Meter value={value} max={max} label={label} kind="progress" showValue={false} tone={tone} />
    </span>
  ),
)
Progress.displayName = 'Progress'

export { Progress }
