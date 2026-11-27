import { ReactNode } from 'react'
import { ChevronRightIcon } from '@/components/icons'
import { Link } from 'react-router-dom'
import { cn } from '@/lib/utils'

interface Breadcrumb {
  label: string
  href?: string
}

interface PageHeaderProps {
  title: string
  description?: string
  action?: ReactNode
  breadcrumbs?: Breadcrumb[]
  className?: string
}

export function PageHeader({ 
  title, 
  description, 
  action, 
  breadcrumbs,
  className 
}: PageHeaderProps) {
  return (
    <div className={cn('border-b border-hairline', className)}>
      <div className="flex flex-col gap-4 p-6">
        {breadcrumbs && breadcrumbs.length > 0 && (
          <nav aria-label="Breadcrumb" className="flex items-center space-x-1 text-sm text-ink-2">
            {breadcrumbs.map((crumb, index) => (
              <div key={index} className="flex items-center">
                {index > 0 && <ChevronRightIcon className="h-4 w-4 mx-1" />}
                {crumb.href ? (
                  <Link 
                    to={crumb.href} 
                    className="underline decoration-ink-2 underline-offset-4 hover:text-ink hover:decoration-ink"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span className="text-ink">{crumb.label}</span>
                )}
              </div>
            ))}
          </nav>
        )}
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h1 className="screen-title">{title}</h1>
            {description && (
              <p className="text-ink-2">{description}</p>
            )}
          </div>
          {action && <div>{action}</div>}
        </div>
      </div>
    </div>
  )
}
