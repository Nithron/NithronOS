import type { ReactNode } from 'react'
import { StatusPill as Pill } from '@/components/ui/status'

interface StatusPillProps {
  children: ReactNode
  status?: 'success' | 'warning' | 'error' | 'info' | 'default'
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

/** The same pill as components/ui/status (a mark, a word, a status line). */
export function StatusPill({ children, status = 'default', size = 'md', className }: StatusPillProps) {
  return (
    <Pill variant={status === 'default' ? 'muted' : status} size={size} className={className}>
      {children}
    </Pill>
  )
}
