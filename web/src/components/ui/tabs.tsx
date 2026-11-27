import * as React from 'react'
import { cn } from '@/lib/utils'

interface TabsContextValue {
  value: string
  onValueChange: (value: string) => void
}

const TabsContext = React.createContext<TabsContextValue | undefined>(undefined)

interface TabsProps {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  children: React.ReactNode
  className?: string
}

export function Tabs({ value: controlledValue, defaultValue, onValueChange, children, className }: TabsProps) {
  const [uncontrolledValue, setUncontrolledValue] = React.useState(defaultValue || '')
  const isControlled = controlledValue !== undefined
  const value = isControlled ? controlledValue : uncontrolledValue
  
  const handleValueChange = React.useCallback((newValue: string) => {
    if (!isControlled) {
      setUncontrolledValue(newValue)
    }
    onValueChange?.(newValue)
  }, [isControlled, onValueChange])
  
  return (
    <TabsContext.Provider value={{ value, onValueChange: handleValueChange }}>
      <div className={cn('w-full', className)}>{children}</div>
    </TabsContext.Provider>
  )
}

interface TabsListProps {
  children: React.ReactNode
  className?: string
}

export function TabsList({ children, className }: TabsListProps) {
  return (
    <div role="tablist" className={cn('flex items-end gap-6 border-b border-hairline', className)}>
      {children}
    </div>
  )
}

interface TabsTriggerProps {
  value: string
  children: React.ReactNode
  className?: string
  disabled?: boolean
}

export function TabsTrigger({ value, children, className, disabled }: TabsTriggerProps) {
  const context = React.useContext(TabsContext)
  if (!context) throw new Error('TabsTrigger must be used within Tabs')

  const isSelected = context.value === value

  return (
    <button
      type="button"
      role="tab"
      aria-selected={isSelected}
      onClick={() => !disabled && context.onValueChange(value)}
      disabled={disabled}
      // A row of words over one hairline; the current one in ink at 560
      // with a 2 px ink mark under it.
      className={cn(
        'relative -mb-px inline-flex min-h-10 items-center whitespace-nowrap py-2 text-sm transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink disabled:pointer-events-none disabled:opacity-50',
        isSelected
          ? 'font-[560] text-ink after:absolute after:inset-x-0 after:bottom-0 after:h-[2px] after:bg-ink'
          : 'text-ink-2 hover:text-ink',
        className
      )}
    >
      {children}
    </button>
  )
}

interface TabsContentProps {
  value: string
  children: React.ReactNode
  className?: string
}

export function TabsContent({ value, children, className }: TabsContentProps) {
  const context = React.useContext(TabsContext)
  if (!context) throw new Error('TabsContent must be used within Tabs')

  if (context.value !== value) return null

  return (
    <div
      className={cn(
        'mt-6 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ink',
        className
      )}
    >
      {children}
    </div>
  )
}
