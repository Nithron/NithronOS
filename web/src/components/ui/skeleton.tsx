import * as React from 'react'
import { cn, flatPanel } from '@/lib/utils'

/**
 * Where data will stand while it loads: the space it takes, with a block
 * cursor that blinks at its start. No grey box, no shimmer.
 */
function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div aria-hidden className={cn('flex items-center', flatPanel(className))} {...props}>
      <span className="pixel-blink inline-block h-[1.1em] w-[0.6em] bg-ink-2" />
    </div>
  )
}

export { Skeleton }
