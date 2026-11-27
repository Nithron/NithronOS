import animate from 'tailwindcss-animate'
import type { Config } from 'tailwindcss'

// A colour from a CSS variable that still takes Tailwind's opacity modifier
// (bg-primary/10): a bare var() would make Tailwind 3 drop those classes.
const token = (name: string): string =>
	`color-mix(in srgb, var(${name}) calc(<alpha-value> * 100%), transparent)`

export default {
	darkMode: ['class'],
	content: ['./index.html', './src/**/*.{ts,tsx}'],
	theme: {
		extend: {
			colors: {
				// The shadcn names (src/styles.css maps them onto the tokens).
				background: token('--background'),
				foreground: token('--foreground'),
				card: { DEFAULT: token('--card'), foreground: token('--card-foreground') },
				popover: { DEFAULT: token('--popover'), foreground: token('--popover-foreground') },
				primary: { DEFAULT: token('--primary'), foreground: token('--primary-foreground') },
				secondary: { DEFAULT: token('--secondary'), foreground: token('--secondary-foreground') },
				muted: { DEFAULT: token('--muted'), foreground: token('--muted-foreground') },
				accent: { DEFAULT: token('--accent'), foreground: token('--accent-foreground') },
				destructive: { DEFAULT: token('--destructive'), foreground: token('--destructive-foreground') },
				border: token('--border'),
				input: token('--input'),
				ring: token('--ring'),
				// The tokens by their own names.
				page: token('--page'),
				ink: { DEFAULT: token('--ink'), 2: token('--ink-2') },
				dim: token('--dim'),
				hairline: token('--hairline'),
				rule: token('--rule'),
				edge: token('--edge'),
				status: {
					ok: token('--status-ok'),
					'ok-text': token('--status-ok-text'),
					'ok-tint': token('--status-ok-tint'),
					warning: token('--status-warning'),
					'warning-text': token('--status-warning-text'),
					'warning-tint': token('--status-warning-tint'),
					failed: token('--status-failed'),
					'failed-text': token('--status-failed-text'),
					'failed-tint': token('--status-failed-tint'),
				},
				// These names once held single brand values, which replaced
				// Tailwind's scales: text-blue-500 and the like produce no CSS.
				// They stay that way (no hue in the interface) until the last
				// such class is gone; the lint rule refuses new ones.
				cyan: 'transparent',
				blue: 'transparent',
				lime: 'transparent',
			},
			fontFamily: {
				sans: ['var(--font-sans)'],
				mono: ['var(--font-mono)'],
			},
			borderRadius: {
				lg: 'var(--radius-control)',
				md: 'calc(var(--radius-control) - 2px)',
				sm: 'var(--radius-small)',
			},
		},
	},
	plugins: [animate],
} satisfies Config
