import React from 'react'
import { cn } from '@/lib/utils'
import { ChevronRightIcon } from '@/components/icons'

interface Breadcrumb {
  label: string
  href?: string
}

interface PageHeaderProps {
  title: string
  description?: string
  breadcrumbs?: Breadcrumb[]
  actions?: React.ReactNode
  className?: string
  icon?: React.ComponentType<{ className?: string }>
}

export function PageHeader({
  title,
  description,
  breadcrumbs,
  actions,
  className,
  icon: Icon,
}: PageHeaderProps) {
  return (
    <div className={cn('mb-6', className)}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav aria-label="Breadcrumb" className="mb-2 flex items-center space-x-1 text-sm text-ink-2">
          {breadcrumbs.map((crumb, index) => (
            <React.Fragment key={index}>
              {index > 0 && <ChevronRightIcon className="h-4 w-4" />}
              {crumb.href ? (
                <a
                  href={crumb.href}
                  className="underline decoration-ink-2 underline-offset-4 hover:text-ink hover:decoration-ink"
                >
                  {crumb.label}
                </a>
              ) : (
                <span className="text-ink">{crumb.label}</span>
              )}
            </React.Fragment>
          ))}
        </nav>
      )}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {Icon && <Icon className="h-8 w-8 text-ink-2" />}
          <div>
            <h1 className="screen-title">{title}</h1>
            {description && (
              <p className="mt-2 text-ink-2">{description}</p>
            )}
          </div>
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </div>
  )
}
