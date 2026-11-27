import { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { Button } from './button'
import { EmptyIcon, NotFoundIcon, SearchIcon, UnreachableIcon, type IconComponent } from '@/components/icons'

type EmptyStateVariant = 'no-data' | 'filtered' | 'error' | 'backend-down'

interface EmptyStateProps {
  variant?: EmptyStateVariant
  icon?: IconComponent
  title: string
  description?: string
  action?: ReactNode | {
    label: string
    onClick: () => void
  }
  className?: string
  /** 16 px inside a group, 24 px when it is the whole screen. */
  size?: 'group' | 'screen'
}

const variantConfig: Record<
  EmptyStateVariant,
  { icon: IconComponent; defaultTitle: string }
> = {
  'no-data': {
    icon: EmptyIcon,
    defaultTitle: 'No data yet',
  },
  'filtered': {
    icon: SearchIcon,
    defaultTitle: 'No results found',
  },
  'error': {
    icon: NotFoundIcon,
    defaultTitle: 'Something went wrong',
  },
  'backend-down': {
    icon: UnreachableIcon,
    defaultTitle: 'Backend unreachable',
  },
}

export function EmptyState({
  variant = 'no-data',
  icon,
  title,
  description,
  action,
  className,
  size = 'group',
}: EmptyStateProps) {
  const config = variantConfig[variant]
  // No picture until the shared pixel pictures arrive: the sentence and its
  // one action, left-aligned. `icon` is kept for callers and not drawn.
  void icon

  return (
    <div
      className={cn(
        'flex flex-col items-start py-12',
        className
      )}
    >
      <h3 className={cn('pixel-display mb-2', size === 'screen' && 'pixel-24')}>{title || config.defaultTitle}</h3>
      {description && (
        <p className="mb-6 max-w-sm text-sm text-ink-2">
          {description}
        </p>
      )}
      {action && (
        typeof action === 'object' && 'label' in action ? (
          <Button onClick={action.onClick} size="sm">
            {action.label}
          </Button>
        ) : (
          action
        )
      )}
    </div>
  )
}
