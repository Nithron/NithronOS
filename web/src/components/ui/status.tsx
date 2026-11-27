import React from 'react'
import { cn } from '@/lib/utils'
import { DotIcon, FailedIcon, NoticeIcon, OkIcon, WarningIcon } from '@/components/icons'

type StatusVariant = 'success' | 'warning' | 'error' | 'info' | 'muted'
type StatusValue = 'active' | 'inactive' | 'running' | 'stopped' | 'idle'

interface StatusPillProps {
  variant?: StatusVariant
  status?: StatusValue | string
  size?: 'sm' | 'md' | 'lg'
  children?: React.ReactNode
  className?: string
}

const statusConfig = {
  success: {
    className: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    icon: OkIcon,
  },
  warning: {
    className: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    icon: WarningIcon,
  },
  error: {
    className: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    icon: FailedIcon,
  },
  info: {
    className: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
    icon: NoticeIcon,
  },
  muted: {
    className: 'bg-muted text-muted-foreground',
    icon: DotIcon,
  },
}

const statusToVariant = (status: string): StatusVariant => {
  switch (status) {
    case 'active':
    case 'running':
    case 'healthy':
      return 'success'
    case 'inactive':
    case 'stopped':
      return 'muted'
    case 'error':
    case 'critical':
      return 'error'
    case 'warning':
    case 'degraded':
      return 'warning'
    case 'idle':
      return 'info'
    default:
      return 'muted'
  }
}

export function StatusPill({ variant, status, size = 'md', children, className }: StatusPillProps) {
  const finalVariant = variant || (status ? statusToVariant(status) : 'muted')
  const config = statusConfig[finalVariant]
  const Icon = config.icon

  const sizeClasses = {
    sm: 'px-2 py-0.5 text-xs',
    md: 'px-2.5 py-0.5 text-xs',
    lg: 'px-3 py-1 text-sm'
  }

  const displayText = children || (status ? status : '')

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full font-medium',
        sizeClasses[size],
        config.className,
        className
      )}
    >
      <Icon className={cn(
        size === 'sm' ? 'h-3 w-3' : size === 'lg' ? 'h-4 w-4' : 'h-3 w-3'
      )} />
      {displayText}
    </span>
  )
}

interface HealthBadgeProps {
  status: 'healthy' | 'degraded' | 'critical' | 'unknown'
  className?: string
}

export function HealthBadge({ status, className }: HealthBadgeProps) {
  const variant = 
    status === 'healthy' ? 'success' :
    status === 'degraded' ? 'warning' :
    status === 'critical' ? 'error' :
    'muted'

  return (
    <StatusPill variant={variant} className={className}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </StatusPill>
  )
}

interface MetricProps {
  label: string
  value: string | number
  delta?: string
  deltaType?: 'increase' | 'decrease' | 'neutral'
  sublabel?: string
  className?: string
}

export function Metric({
  label,
  value,
  delta,
  deltaType = 'neutral',
  sublabel,
  className,
}: MetricProps) {
  return (
    <div className={cn('space-y-1', className)}>
      <p className="text-sm text-muted-foreground">{label}</p>
      <div className="flex items-baseline gap-2">
        <p className="text-2xl font-bold">{value}</p>
        {delta && (
          <span
            className={cn(
              'text-sm font-medium',
              deltaType === 'increase' && 'text-green-600 dark:text-green-400',
              deltaType === 'decrease' && 'text-red-600 dark:text-red-400',
              deltaType === 'neutral' && 'text-muted-foreground'
            )}
          >
            {delta}
          </span>
        )}
      </div>
      {sublabel && (
        <p className="text-xs text-muted-foreground">{sublabel}</p>
      )}
    </div>
  )
}
