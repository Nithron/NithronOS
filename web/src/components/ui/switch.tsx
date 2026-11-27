import { forwardRef, ElementRef, ComponentPropsWithoutRef } from 'react'
import * as SwitchPrimitives from '@radix-ui/react-switch'
import { cn } from '@/lib/utils'

const Switch = forwardRef<
  ElementRef<typeof SwitchPrimitives.Root>,
  ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitives.Root
    className={cn(
      'peer inline-flex h-6 w-11 shrink-0 cursor-pointer items-center border border-edge p-[3px] transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:border-ink data-[state=checked]:bg-ink',
      className
    )}
    {...props}
    ref={ref}
  >
    <SwitchPrimitives.Thumb
      className={cn(
        'pointer-events-none block h-4 w-4 transition-transform data-[state=checked]:translate-x-5 data-[state=checked]:bg-page data-[state=unchecked]:translate-x-0 data-[state=unchecked]:bg-ink-2'
      )}
    />
  </SwitchPrimitives.Root>
))
Switch.displayName = SwitchPrimitives.Root.displayName

export { Switch }
