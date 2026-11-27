import type { ReactNode } from 'react'
import { CloseIcon } from '@/components/icons'
import { cn } from '@/lib/utils'

interface ChipProps {
  children: ReactNode
  /** Names the chip for the remove button ("Remove alice"). */
  name: string
  onRemove?: () => void
  className?: string
}

/**
 * A chosen item in a picker (a user, a group, a network): a hairline box
 * with 3 px corners and ink text. Its remove button has its own name.
 */
export function Chip({ children, name, onRemove, className }: ChipProps) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 rounded-sm border border-rule px-2 py-0.5 text-sm text-ink', className)}>
      {children}
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={`Remove ${name}`}
          className="-mr-1 p-0.5 text-ink-2 hover:bg-ink/[.09] hover:text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-ink"
        >
          <CloseIcon className="h-3 w-3" />
        </button>
      )}
    </span>
  )
}
