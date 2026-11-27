import { test, expect, type Page } from '@playwright/test'
import * as fs from 'node:fs'
import * as path from 'node:path'

// The live UI pass (scripts/live-ui/run.sh): against a real nosd and
// nos-agent, sign in and open every route of the web UI's router. A page
// passes when it renders without a script
// error, without a console error and without an API call failing with a
// server error or a missing route.

// The walk saves its own screenshots; no video (and no ffmpeg) needed.
test.use({ video: 'off', trace: 'off' })

const user = process.env.NOS_USER ?? 'admin'
const pass = process.env.NOS_PASS ?? ''

// Every route in the web UI's router (web/src/App.tsx), nested paths
// joined. Routes with a parameter (":id") are filled from live data below.
function routedPaths(appSource: string): string[] {
	const paths: string[] = []
	const prefixes: { depth: number; prefix: string }[] = [{ depth: -1, prefix: '' }]
	let depth = 0
	let last = ''
	const token = /path:\s*'([^']*)'|children:\s*\[|\[|\]/g
	for (const m of appSource.matchAll(token)) {
		const prefix = prefixes[prefixes.length - 1].prefix
		if (m[1] !== undefined) {
			last = m[1].startsWith('/') ? m[1] : `${prefix.replace(/\/$/, '')}/${m[1]}`
			paths.push(last)
		} else if (m[0].startsWith('children')) {
			prefixes.push({ depth, prefix: last })
			depth++
		} else if (m[0] === '[') {
			depth++
		} else {
			depth--
			if (depth <= prefixes[prefixes.length - 1].depth) prefixes.pop()
		}
	}
	return [...new Set(paths)]
}

// Routes the walk can't open, and why.
const notWalked: Record<string, string> = {
	'/*': 'the not-found page; /no-such-page is opened instead',
	'/apps/:id': "an installed app's page; the live container has no Docker to install one",
}

interface PageReport {
	path: string
	pageErrors: string[]
	consoleErrors: string[]
	failedCalls: string[]
	heading: string
}

async function signIn(page: Page) {
	await page.goto('/login')
	await page.locator('#username').waitFor({ timeout: 20_000 })
	await page.locator('#username').fill(user)
	await page.locator('#password').fill(pass)
	await page.locator('button[type="submit"]').first().click()
	await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 15_000 })
}

test('every page renders against a real nosd', async ({ page }) => {
	test.setTimeout(8 * 60_000)
	expect(pass, 'NOS_PASS').not.toBe('')
	await signIn(page)

	const reports: PageReport[] = []
	let current: PageReport | null = null
	page.on('pageerror', (e) => current?.pageErrors.push(e.message))
	page.on('console', (m) => {
		if (m.type() === 'error') current?.consoleErrors.push(m.text().slice(0, 300))
	})
	page.on('response', (r) => {
		const url = new URL(r.url())
		// 404/405 (no such route) and 5xx are failures; 4xx answers to a
		// request the page made on purpose (validation) are not.
		if (url.pathname.startsWith('/api/') && (r.status() >= 500 || r.status() === 404 || r.status() === 405)) {
			current?.failedCalls.push(`${r.request().method()} ${url.pathname}${url.search} -> ${r.status()}`)
		}
	})

	// Every route; a route with a parameter opens the first thing of its kind
	// (the share the sync pass made, the first pool, the first catalog app).
	// Asked from the page: the session cookie is Secure, which Playwright's
	// own request client won't send over http, while the browser does on
	// 127.0.0.1.
	const get = (url: string) =>
		page.evaluate((u) => fetch(u).then((r) => (r.ok ? r.json() : null)).catch(() => null), url)
	const pools = await get('/api/v1/pools')
	const shares = await get('/api/v1/shares')
	const catalog = await get('/api/v1/apps/catalog')
	const first = (list: unknown, ...keys: string[]): string | undefined => {
		const item = Array.isArray(list) ? list[0] : undefined
		for (const k of keys) if (item?.[k]) return String(item[k])
		return undefined
	}
	const params: Record<string, string | undefined> = {
		'/storage/:id': first(pools, 'id', 'uuid'),
		'/shares/:name': first(shares, 'name'),
		'/apps/install/:id': first(catalog?.entries, 'id'),
	}
	const appSource = fs.readFileSync(path.resolve(__dirname, '../../../../web/src/App.tsx'), 'utf8')
	const pages: string[] = ['/no-such-page']
	const missing: string[] = []
	for (const route of routedPaths(appSource)) {
		if (route in notWalked) continue
		if (!route.includes(':')) {
			pages.push(route)
		} else if (params[route]) {
			pages.push(route.replace(/:[^/]+/, encodeURIComponent(params[route]!)))
		} else {
			missing.push(route)
		}
	}
	// A new route with a parameter needs a value above (or a reason in
	// notWalked); a known one without data means the setup before the walk
	// broke.
	expect(missing, 'routes with nothing to open').toEqual([])

	for (const p of pages) {
		current = { path: p, pageErrors: [], consoleErrors: [], failedCalls: [], heading: '' }
		await page.goto(p)
		await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {})
		await page.waitForTimeout(500)
		current.heading = (await page.locator('h1').first().textContent({ timeout: 2_000 }).catch(() => null))?.trim() ?? '(no h1)'
		const file = `test-results/live/${p.replace(/\//g, '_') || '_root'}`
		await page.screenshot({ path: `${file}.png`, fullPage: true })
		// The visible text, for reviewing what a page claims (error cards,
		// placeholders, made-up numbers).
		fs.mkdirSync('test-results/live', { recursive: true })
		fs.writeFileSync(`${file}.txt`, await page.locator('main').first().innerText({ timeout: 2_000 }).catch(() => page.locator('body').innerText()))
		reports.push(current)
	}
	current = null

	fs.mkdirSync('test-results', { recursive: true })
	fs.writeFileSync('live-report.json', JSON.stringify(reports, null, 2))
	const broken = reports.filter((r) => r.pageErrors.length || r.consoleErrors.length || r.failedCalls.length)
	for (const r of broken) {
		console.log(`\n${r.path} [${r.heading}]`)
		for (const e of r.pageErrors) console.log(`  script error: ${e}`)
		for (const e of r.consoleErrors) console.log(`  console: ${e}`)
		for (const e of r.failedCalls) console.log(`  api: ${e}`)
	}
	console.log(`\n${reports.length - broken.length}/${reports.length} pages clean`)
	expect(broken.map((r) => r.path)).toEqual([])
})
