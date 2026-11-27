#!/usr/bin/env node
// The interface's own rules (docs/dev/ui-identity-proposal.md), checked on
// every file in src/:
//   - icons come from src/components/icons.ts only, never from lucide-react;
//   - no raw colour: no Tailwind palette class (text-gray-400, bg-white,
//     ...), no hex value, no dark: variant (the tokens switch the theme),
//     no shadow, gradient or blur.
// Files written before the rework keep their raw colours until their area
// is converted: scripts/ui-legacy.json lists how many each may still have,
// and the count may only go down. A new file has none.
//
//   node scripts/check-ui.mjs           check
//   node scripts/check-ui.mjs --update  rewrite the legacy counts (only
//                                       ever lower them)
//
// A line that must hold a fixed colour (a QR code is black on white in
// both themes) says so with the comment `ui-check: fixed colour`.
import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const WEB = join(fileURLToPath(import.meta.url), '..', '..')
const SRC = join(WEB, 'src')
const LEGACY = join(WEB, 'scripts', 'ui-legacy.json')
const ICONS = 'src/components/icons.ts'

const HUES =
  'slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose'
const UTILS =
  'text|bg|border|border-[trblxy]|ring|ring-offset|from|via|to|fill|stroke|divide|outline|placeholder|decoration|accent|caret'
const RAW = [
  new RegExp(`(?<![\\w-])(?:[\\w-]+:)*(?:${UTILS})-(?:${HUES})-\\d{2,3}(?:/\\d+)?(?![\\w-])`, 'g'),
  new RegExp(`(?<![\\w-])(?:[\\w-]+:)*(?:${UTILS})-(?:white|black)(?:/\\d+)?(?![\\w-])`, 'g'),
  /(?<![\w&-])#[0-9a-fA-F]{6}\b|(?<![\w&-])#[0-9a-fA-F]{3}\b(?![\w-])/g,
  /(?<![\w-])dark:[\w-[\]/.]+/g,
  /(?<![\w-])(?:[\w-]+:)*shadow(?:-(?:sm|md|lg|xl|2xl|inner))?(?![\w-])/g,
  /(?<![\w-])bg-gradient-to-[a-z]+/g,
  /(?<![\w-])(?:backdrop-)?blur(?:-[a-z0-9]+)?(?![\w-])/g,
]
const FIXED = 'ui-check: fixed colour'

function files(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return files(p)
    return /\.(ts|tsx)$/.test(e.name) && !/\.(test|spec)\.tsx?$/.test(e.name) ? [p] : []
  })
}

function rawCount(text) {
  return text
    .split('\n')
    .filter((line) => !line.includes(FIXED))
    .reduce((n, line) => n + RAW.reduce((m, re) => m + (line.match(re)?.length ?? 0), 0), 0)
}

const counts = {}
const problems = []
for (const path of files(SRC)) {
  const rel = relative(WEB, path)
  const text = readFileSync(path, 'utf8')
  if (rel !== ICONS && /from\s+['"]lucide-react['"]/.test(text)) {
    problems.push(`${rel}: imports lucide-react; import the icon from '@/components/icons' by its meaning`)
  }
  const n = rawCount(text)
  if (n > 0) counts[rel] = n
}

const legacy = JSON.parse(readFileSync(LEGACY, 'utf8'))
if (process.argv.includes('--update')) {
  const next = {}
  for (const [rel, n] of Object.entries(counts)) {
    if (!(rel in legacy) || n > legacy[rel]) {
      console.error(`${rel}: ${n} raw colours, not more than before; --update only lowers counts`)
      process.exit(1)
    }
    next[rel] = n
  }
  writeFileSync(LEGACY, JSON.stringify(next, null, 2) + '\n')
  console.log(`ui-legacy.json: ${Object.keys(next).length} files, ${Object.values(next).reduce((a, b) => a + b, 0)} raw colours left`)
  process.exit(0)
}

for (const [rel, n] of Object.entries(counts)) {
  const allowed = legacy[rel] ?? 0
  if (n > allowed) {
    problems.push(
      `${rel}: ${n} raw colours (allowed ${allowed}); use the tokens (bg-page, text-ink-2, text-status-failed-text, ...)`,
    )
  }
}
const lowered = Object.entries(legacy).filter(([rel, n]) => (counts[rel] ?? 0) < n)
if (problems.length) {
  console.error(problems.join('\n'))
  process.exit(1)
}
const left = Object.values(counts).reduce((a, b) => a + b, 0)
console.log(`ui check: ok (${left} raw colours left in ${Object.keys(counts).length} files written before the rework)`)
if (lowered.length) console.log(`fewer than listed in ${lowered.length} files: run with --update to lower the counts`)
