import { ReactNode } from 'react'
import { cn } from '@/lib/utils'

interface BadgeProps {
  children: ReactNode
  variant?: 'default' | 'outline' | 'secondary' | 'destructive' | 'success'
  className?: string
}

export function Badge({ children, variant = 'default', className }: BadgeProps) {
  // A badge is a hairline box with ink text; a state's colour is its line
  // and tint, beside its mark (pass one in children), never a fill.
  const variants = {
    default: 'border-rule',
    outline: 'border-rule',
    secondary: 'border-rule',
    destructive: 'border-status-failed bg-status-failed-tint',
    success: 'border-status-ok bg-status-ok-tint',
  }
  
  return (
    <span 
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm border px-2 py-0.5 text-xs font-[560] text-ink',
        variants[variant],
        className
      )}
    >
      {children}
    </span>
  )
}


