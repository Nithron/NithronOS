// Fails when the web UI calls an API path that nosd doesn't serve.
//
// Every string or template literal in web/src that starts with /api/v1/ or
// /v1/ is matched against docs/api/openapi.json (generated from the router).
// Paths that are known to be missing today are listed in
// docs/api/fe-known-missing.json; the check fails on any new one, and on
// listed ones that now exist (remove them from the list).
//
//   node scripts/check-fe-api-paths.mjs           check
//   node scripts/check-fe-api-paths.mjs --write   rewrite the known-missing list
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const specPath = path.join(root, 'docs/api/openapi.json')
const knownPath = path.join(root, 'docs/api/fe-known-missing.json')
const srcDir = path.join(root, 'web/src')

const skip = (f) =>
  /__tests__|\.test\.|\.spec\.|\/mocks?\//.test(f) || f.endsWith('api/schema.ts') || f.endsWith('.d.ts')

function sourceFiles(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) return sourceFiles(p)
    return /\.(ts|tsx|js|jsx)$/.test(e.name) && !skip(p) ? [p] : []
  })
}

// '/v1/apps/${id}/logs?tail=5' -> '/api/v1/apps/{}/logs'
function normalize(p) {
  p = p.replace(/\$\{[^}]*\}/g, '{}').split(/[?#]/)[0]
  // '/v1/backup/jobs${query}': a parameter glued to a segment is a query string
  p = p.replace(/([^/])\{\}/g, '$1')
  if (p.startsWith('/v1/')) p = '/api' + p
  return p
}

const spec = JSON.parse(fs.readFileSync(specPath, 'utf-8'))
const routes = Object.keys(spec.paths).map((p) => p.split('/'))

function served(p) {
  const prefix = p.endsWith('/') // a base for concatenation: any route under it
  const segs = p.replace(/\/+$/, '').split('/')
  return routes.some((r) => {
    if (prefix ? r.length < segs.length : r.length !== segs.length) return false
    return segs.every((s, i) => s === '{}' || r[i].startsWith('{') || s === r[i])
  })
}

const literal = /(['"`])(\/(?:api\/)?v1\/[^'"`\s]*)\1/g
// `const BASE_PATH = '/v1/sync'`: a base other paths are joined to, served
// when any route is below it.
const baseConst = /\b(?:const|let|var)\s+\w*(?:BASE|PREFIX|ROOT)\w*\s*=\s*$/i
// A client call (nos-client prefixes /api) with a path outside /v1: nosd
// serves nothing there. (PoolDetails listed pools from /api/pools.)
const unversionedCall = /\b(?:http|httpCore|api|nos)\.(?:get|post|put|patch|del|delete)\s*(?:<[^>()]*>)?\(\s*(['"`])(\/(?!v1\/)[^'"`\s]*)\1/g
const missing = new Map() // path -> files
for (const file of sourceFiles(srcDir)) {
  const text = fs.readFileSync(file, 'utf-8')
  for (const m of text.matchAll(literal)) {
    let p = normalize(m[2])
    const before = text.slice(Math.max(0, m.index - 80), m.index)
    if (baseConst.test(before) && !p.endsWith('/')) p += '/'
    if (served(p)) continue
    const rel = path.relative(root, file)
    missing.set(p, [...new Set([...(missing.get(p) || []), rel])].sort())
  }
  for (const m of text.matchAll(unversionedCall)) {
    const p = '/api' + m[2].replace(/\$\{[^}]*\}/g, '{}').replace(/\?.*$/, '')
    const rel = path.relative(root, file)
    missing.set(p, [...new Set([...(missing.get(p) || []), rel])].sort())
  }
}

const found = [...missing.keys()].sort().map((p) => ({ path: p, files: missing.get(p) }))
if (process.argv.includes('--write')) {
  fs.writeFileSync(knownPath, JSON.stringify(found, null, 2) + '\n')
  console.log(`wrote ${found.length} known-missing paths to ${path.relative(root, knownPath)}`)
  process.exit(0)
}

const known = new Set(JSON.parse(fs.readFileSync(knownPath, 'utf-8')).map((k) => k.path))
const added = found.filter((f) => !known.has(f.path))
const fixed = [...known].filter((p) => !missing.has(p))
for (const f of added) console.error(`UI calls ${f.path}, which nosd doesn't serve (${f.files.join(', ')})`)
for (const p of fixed) console.error(`${p} is served now (or no longer called): remove it from docs/api/fe-known-missing.json`)
if (added.length || fixed.length) process.exit(1)
console.log(`UI API paths: all served, except ${known.size} known gaps (docs/api/fe-known-missing.json)`)
