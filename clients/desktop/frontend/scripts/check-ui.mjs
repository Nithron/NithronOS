// The identity's lint rule for NithronSync (as web/scripts/check-ui.mjs is for
// nOS): no hue outside the tokens file, no second icon set, no shadow,
// gradient, glow or blur, no dark: variants (the tokens follow the system).
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const root = new URL('..', import.meta.url).pathname
const skip = new Set(['styles/nithron-tokens.css', 'vendor'])
const rules = [
  [/lucide-react|react-icons|@heroicons/, 'a second icon set'],
  [/#[0-9a-fA-F]{3,8}\b/, 'a hex colour'],
  [/\b(?:bg|text|border|ring|fill|stroke|from|to|via|divide|outline)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black)\b/, 'a raw palette class'],
  [/\bdark:/, 'a dark: variant'],
  [/\b(?:shadow|drop-shadow|blur|backdrop-blur)(?:-[\w[\]]+)?\b|box-shadow|text-shadow|gradient/, 'a shadow, gradient or blur'],
  [/\banimate-spin\b/, 'a spinner'],
]

const files = []
const walk = (dir) => {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const rel = p.slice(join(root, 'src').length + 1)
    if (skip.has(rel) || skip.has(rel.split('/')[0])) continue
    if (statSync(p).isDirectory()) walk(p)
    else if (/\.(tsx?|css)$/.test(name)) files.push(p)
  }
}
walk(join(root, 'src'))
files.push(join(root, 'index.html'), join(root, 'tailwind.config.js'))

let problems = 0
for (const f of files) {
  readFileSync(f, 'utf8').split('\n').forEach((line, i) => {
    for (const [re, what] of rules) {
      if (re.test(line)) {
        console.log(`${f.slice(root.length)}:${i + 1}: ${what}: ${line.trim()}`)
        problems++
      }
    }
  })
}
if (problems) {
  console.log(`ui check: ${problems} problem(s)`)
  process.exit(1)
}
console.log(`ui check: ok (${files.length} files)`)
