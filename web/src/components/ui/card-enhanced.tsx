import React from 'react'
import { cn, flatPanel } from '@/lib/utils'
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
      className={cn('border border-hairline bg-page', flatPanel(className))}
    >
      {(title || actions) && (
        <div className="flex items-center justify-between border-b border-hairline px-6 py-4">
          <div className="flex items-center gap-2">
            {title && (
              <div>
                <h3 className="text-xl font-[560] tracking-[-0.012em]">{title}</h3>
                {description && (
                  <p className="text-sm text-ink-2">{description}</p>
                )}
              </div>
            )}
            {helpTooltip && (
              <div className="group relative">
                <HelpIcon className="h-4 w-4 text-ink-2" />
                <div className="absolute left-0 top-full z-10 mt-1 hidden w-64 border border-edge bg-page p-2 text-sm text-ink group-hover:block">
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
          <div className="py-8" role="alert">
            <p className="mb-4 flex items-center gap-2 text-sm text-ink">
              <FailedIcon className="h-4 w-4 text-status-failed" />
              {error}
            </p>
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
        <div className="border-t border-hairline px-6 py-4">{footer}</div>
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
        'grid gap-px border border-hairline bg-hairline md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 [&>*]:border-0',
        className
      )}
    >
      {children}
    </div>
  )
}
