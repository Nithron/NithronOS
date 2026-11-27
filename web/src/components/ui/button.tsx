import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const buttonVariants = cva(
	// 8 px corners, weight 560, the focus an ink outline. The first action
	// is ink; every other one is a line button. A destructive action is a
	// line button too: its words say what is destroyed, never a red fill.
	'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-[0.9375rem] font-[560] transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ink disabled:pointer-events-none disabled:opacity-50',
	{
		variants: {
			variant: {
				default: 'bg-ink text-page hover:bg-ink/85',
				destructive: 'border border-rule text-ink hover:border-ink',
				outline: 'border border-rule text-ink hover:border-ink',
				secondary: 'border border-rule text-ink hover:border-ink',
				ghost: 'text-ink hover:bg-ink/[.09]',
				link: 'text-ink underline decoration-ink-2 underline-offset-4 hover:decoration-ink',
			},
			size: {
				default: 'h-11 px-5',
				sm: 'h-8 px-3 text-sm',
				lg: 'h-11 px-6',
				icon: 'h-8 w-8',
			},
		},
		defaultVariants: {
			variant: 'default',
			size: 'default',
		},
	},
)

export interface ButtonProps
	extends React.ButtonHTMLAttributes<HTMLButtonElement>,
		VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
	({ className, variant, size, ...props }, ref) => {
		return (
			<button className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
		)
	},
)
Button.displayName = 'Button'

export { Button, buttonVariants }


