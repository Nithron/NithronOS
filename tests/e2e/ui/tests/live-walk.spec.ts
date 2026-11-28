import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import * as fs from 'node:fs'
import * as path from 'node:path'

// The live UI pass (scripts/live-ui/run.sh): against a real nosd and
// nos-agent, sign in and open every route of the web UI's router. A page
// passes when it renders without a script
// error, without a console error and without an API call failing with a
// server error or a missing route.
//
// The walk runs twice, in the system's dark and light theme (the tokens
// follow prefers-color-scheme), saves a capture of every page in each, and
// measures the contrast of every text (axe, WCAG AA): no page may have a
// text below AA in either theme. The counts are also written to
// test-results/contrast-report.json.

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
}

const themes = ['dark', 'light'] as const

interface PageReport {
	path: string
	theme: (typeof themes)[number]
	contrast: { count: number; examples: string[] }
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
	test.setTimeout(20 * 60_000)
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
	// (the share the sync pass made, the first pool, the first catalog app,
	// the app the apps pass installed).
	// Asked from the page: the session cookie is Secure, which Playwright's
	// own request client won't send over http, while the browser does on
	// 127.0.0.1.
	const get = (url: string) =>
		page.evaluate((u) => fetch(u).then((r) => (r.ok ? r.json() : null)).catch(() => null), url)
	const pools = await get('/api/v1/pools')
	const shares = await get('/api/v1/shares')
	const catalog = await get('/api/v1/apps/catalog')
	const installed = await get('/api/v1/apps/installed')
	const first = (list: unknown, ...keys: string[]): string | undefined => {
		const item = Array.isArray(list) ? list[0] : undefined
		for (const k of keys) if (item?.[k]) return String(item[k])
		return undefined
	}
	const params: Record<string, string | undefined> = {
		'/storage/:id': first(pools, 'id', 'uuid'),
		'/shares/:name': first(shares, 'name'),
		'/apps/install/:id': first(catalog?.entries, 'id'),
		'/apps/:id': first(installed?.items, 'id'),
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

	for (const theme of themes) {
		await page.emulateMedia({ colorScheme: theme })
		const dir = `test-results/live/${theme}`
		fs.mkdirSync(dir, { recursive: true })
		for (const p of pages) {
			current = {
				path: p, theme, contrast: { count: 0, examples: [] },
				pageErrors: [], consoleErrors: [], failedCalls: [], heading: '',
			}
			await page.goto(p)
			await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {})
			await page.waitForTimeout(500)
			current.heading = (await page.locator('h1').first().textContent({ timeout: 2_000 }).catch(() => null))?.trim() ?? '(no h1)'
			const file = `${dir}/${p.replace(/\//g, '_') || '_root'}`
			await page.screenshot({ path: `${file}.png`, fullPage: true })
			// The visible text, for reviewing what a page claims (error cards,
			// placeholders, made-up numbers); once, it is the same in both themes.
			if (theme === themes[0]) {
				fs.writeFileSync(`${file}.txt`, await page.locator('main').first().innerText({ timeout: 2_000 }).catch(() => page.locator('body').innerText()))
			}
			const axe = await new AxeBuilder({ page }).withRules(['color-contrast']).analyze()
			const nodes = axe.violations.flatMap((v) => v.nodes)
			current.contrast = {
				count: nodes.length,
				examples: nodes.slice(0, 5).map((n) => `${n.target.join(' ')}: ${n.failureSummary?.split('\n').pop()?.trim() ?? ''}`),
			}
			reports.push(current)
		}
	}
	current = null
	await page.emulateMedia({ colorScheme: null })

	fs.mkdirSync('test-results', { recursive: true })
	fs.writeFileSync('live-report.json', JSON.stringify(reports, null, 2))
	const contrast = reports.map((r) => ({ path: r.path, theme: r.theme, below_aa: r.contrast.count, examples: r.contrast.examples }))
	fs.writeFileSync('test-results/contrast-report.json', JSON.stringify(contrast, null, 2))
	for (const theme of themes) {
		const mine = contrast.filter((c) => c.theme === theme)
		console.log(`contrast, ${theme}: ${mine.filter((c) => c.below_aa === 0).length}/${mine.length} pages with every text at AA`)
	}
	const broken = reports.filter((r) => r.pageErrors.length || r.consoleErrors.length || r.failedCalls.length)
	for (const r of broken) {
		console.log(`\n${r.path} (${r.theme}) [${r.heading}]`)
		for (const e of r.pageErrors) console.log(`  script error: ${e}`)
		for (const e of r.consoleErrors) console.log(`  console: ${e}`)
		for (const e of r.failedCalls) console.log(`  api: ${e}`)
	}
	console.log(`\n${reports.length - broken.length}/${reports.length} page views clean`)
	expect(broken.map((r) => `${r.path} (${r.theme})`)).toEqual([])
	const belowAA = reports.filter((r) => r.contrast.count > 0)
	expect(belowAA.map((r) => `${r.path} (${r.theme}): ${r.contrast.examples.join('; ')}`), 'text below AA').toEqual([])
})
