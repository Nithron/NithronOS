import React from 'react'
import { cn } from '@/lib/utils'
import { Mark, type MarkState } from '@/components/ui/pixel'

type StatusVariant = 'success' | 'warning' | 'error' | 'info' | 'muted'
type StatusValue = 'active' | 'inactive' | 'running' | 'stopped' | 'idle'

interface StatusPillProps {
  variant?: StatusVariant
  status?: StatusValue | string
  size?: 'sm' | 'md' | 'lg'
  children?: React.ReactNode
  className?: string
}

// A pill is a 1 px line in the status colour, its tint as fill, ink text
// and 3 px corners, with the state's mark before the word.
const statusConfig: Record<StatusVariant, { className: string; mark: MarkState }> = {
  success: { className: 'border-status-ok bg-status-ok-tint', mark: 'ok' },
  warning: { className: 'border-status-warning bg-status-warning-tint', mark: 'warning' },
  error: { className: 'border-status-failed bg-status-failed-tint', mark: 'failed' },
  info: { className: 'border-rule', mark: 'notice' },
  muted: { className: 'border-rule', mark: 'off' },
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

  const sizeClasses = {
    sm: 'px-2 py-0.5 text-xs',
    md: 'px-2.5 py-0.5 text-xs',
    lg: 'px-3 py-1 text-sm',
  }

  const displayText = children || (status ? status : '')

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm border font-medium text-ink',
        sizeClasses[size],
        config.className,
        className,
      )}
    >
      <Mark state={config.mark} />
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
              // A rise or a fall is no state: the words say which.
              deltaType === 'neutral' ? 'text-ink-2' : 'text-ink'
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
