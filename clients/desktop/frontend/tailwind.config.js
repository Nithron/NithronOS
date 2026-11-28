/** @type {import('tailwindcss').Config} */

// A colour from a token that still takes Tailwind's opacity modifier
// (bg-ink/[.09]): a bare var() would make Tailwind 3 drop those classes.
const token = (name) => `color-mix(in srgb, var(${name}) calc(<alpha-value> * 100%), transparent)`

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        page: token('--page'),
        ink: { DEFAULT: token('--ink'), 2: token('--ink-2') },
        dim: token('--dim'),
        hairline: token('--hairline'),
        rule: token('--rule'),
        edge: token('--edge'),
        status: {
          ok: token('--status-ok'),
          'ok-text': token('--status-ok-text'),
          warning: token('--status-warning'),
          'warning-text': token('--status-warning-text'),
          failed: token('--status-failed'),
          'failed-text': token('--status-failed-text'),
        },
      },
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
      },
      borderRadius: {
        control: 'var(--radius-control)',
      },
    },
  },
  plugins: [],
}
