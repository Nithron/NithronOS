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
        'flex flex-col items-center justify-center py-12 text-center',
        className
      )}
    >
      <div className="mb-4 rounded-full bg-muted/30 p-3">
        <Icon className="h-12 w-12 text-muted-foreground" />
      </div>
      <h3 className="mb-2 text-lg font-semibold">{title}</h3>
      {description && (
        <p className="mb-6 max-w-sm text-sm text-muted-foreground">
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
