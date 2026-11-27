import { forwardRef, ElementRef, ComponentPropsWithoutRef } from 'react'
import * as SwitchPrimitives from '@radix-ui/react-switch'
import { cn } from '@/lib/utils'

const Switch = forwardRef<
  ElementRef<typeof SwitchPrimitives.Root>,
  ComponentPropsWithoutRef<typeof SwitchPrimitives.Root>
>(({ className, ...props }, ref) => (
  // The state is also a word beside the track, never the fill alone.
  <span className="inline-flex items-center gap-2">
    <SwitchPrimitives.Root
      className={cn(
        'peer inline-flex h-6 w-11 shrink-0 cursor-pointer items-center border border-edge p-[3px] transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:border-ink data-[state=checked]:bg-ink',
        className,
      )}
      {...props}
      ref={ref}
    >
      <SwitchPrimitives.Thumb
        className={cn(
          'pointer-events-none block h-4 w-4 transition-transform data-[state=checked]:translate-x-5 data-[state=checked]:bg-page data-[state=unchecked]:translate-x-0 data-[state=unchecked]:bg-ink-2',
        )}
      />
    </SwitchPrimitives.Root>
    <span aria-hidden className="w-6 font-mono text-xs text-ink-2 peer-data-[state=checked]:hidden">
      Off
    </span>
    <span
      aria-hidden
      className="hidden w-6 font-mono text-xs text-ink peer-data-[state=checked]:inline"
    >
      On
    </span>
  </span>
))
Switch.displayName = SwitchPrimitives.Root.displayName

export { Switch }
