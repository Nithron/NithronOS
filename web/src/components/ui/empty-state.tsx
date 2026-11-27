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
}: EmptyStateProps) {
  const config = variantConfig[variant]
  const Icon = icon || config.icon

  return (
    <div
      className={cn(
        'flex flex-col items-start py-12',
        className
      )}
    >
      <Icon className="mb-4 h-8 w-8 text-ink-2" aria-hidden />
      <h3 className="mb-2 text-xl font-[560] tracking-[-0.012em]">{title}</h3>
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
