import React from 'react'
import { cn } from '@/lib/utils'
import { FailedIcon, HelpIcon } from '@/components/icons'
import { Working } from './pixel'
import { Button } from './button'

interface CardProps {
  title?: string
  description?: string
  helpTooltip?: string
  actions?: React.ReactNode
  children: React.ReactNode
  footer?: React.ReactNode
  isLoading?: boolean
  error?: string | null
  onRetry?: () => void
  className?: string
  contentClassName?: string
}

export function Card({
  title,
  description,
  helpTooltip,
  actions,
  children,
  footer,
  isLoading = false,
  error = null,
  onRetry,
  className,
  contentClassName,
}: CardProps) {
  return (
    <div
      className={cn(
        'rounded-2xl border border-border bg-card shadow-sm',
        className
      )}
    >
      {(title || actions) && (
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <div className="flex items-center gap-2">
            {title && (
              <div>
                <h3 className="text-lg font-semibold">{title}</h3>
                {description && (
                  <p className="text-sm text-muted-foreground">{description}</p>
                )}
              </div>
            )}
            {helpTooltip && (
              <div className="group relative">
                <HelpIcon className="h-4 w-4 text-muted-foreground" />
                <div className="absolute left-0 top-full z-10 mt-1 hidden w-64 rounded-lg bg-popover p-2 text-sm text-popover-foreground shadow-lg group-hover:block">
                  {helpTooltip}
                </div>
              </div>
            )}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}

      <div className={cn('p-6', contentClassName)}>
        {isLoading ? (
          <div className="py-8">
            <Working />
          </div>
        ) : error ? (
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <FailedIcon className="mb-4 h-12 w-12 text-destructive" />
            <p className="mb-4 text-sm text-muted-foreground">{error}</p>
            {onRetry && (
              <Button onClick={onRetry} variant="outline" size="sm">
                Retry
              </Button>
            )}
          </div>
        ) : (
          children
        )}
      </div>

      {footer && (
        <div className="border-t border-border px-6 py-4">{footer}</div>
      )}
    </div>
  )
}

interface CardGridProps {
  children: React.ReactNode
  className?: string
}

export function CardGrid({ children, className }: CardGridProps) {
  return (
    <div
      className={cn(
        'grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4',
        className
      )}
    >
      {children}
    </div>
  )
}
