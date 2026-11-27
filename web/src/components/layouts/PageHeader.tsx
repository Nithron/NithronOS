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
    <div className={cn('border-b bg-background', className)}>
      <div className="flex flex-col gap-4 p-6">
        {breadcrumbs && breadcrumbs.length > 0 && (
          <nav className="flex items-center space-x-1 text-sm text-muted-foreground">
            {breadcrumbs.map((crumb, index) => (
              <div key={index} className="flex items-center">
                {index > 0 && <ChevronRightIcon className="h-4 w-4 mx-1" />}
                {crumb.href ? (
                  <Link 
                    to={crumb.href} 
                    className="hover:text-foreground transition-colors"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span className="text-foreground font-medium">{crumb.label}</span>
                )}
              </div>
            ))}
          </nav>
        )}
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            {description && (
              <p className="text-muted-foreground">{description}</p>
            )}
          </div>
          {action && <div>{action}</div>}
        </div>
      </div>
    </div>
  )
}
