import { test, expect, type Page } from '@playwright/test'
import * as fs from 'node:fs'
import * as path from 'node:path'

// The live UI pass (scripts/live-ui/run.sh): against a real nosd and
// nos-agent, sign in and open every page the navigation offers, plus the
// pages reached from them. A page passes when it renders without a script
// error, without a console error and without an API call failing with a
// server error or a missing route.

// The walk saves its own screenshots; no video (and no ffmpeg) needed.
test.use({ video: 'off', trace: 'off' })

const user = process.env.NOS_USER ?? 'admin'
const pass = process.env.NOS_PASS ?? ''

// Every path in the web UI's navigation, plus pages linked from them.
function pagesToVisit(): string[] {
	const nav = fs.readFileSync(path.resolve(__dirname, '../../../../web/src/config/nav.ts'), 'utf8')
	const fromNav = [...nav.matchAll(/path:\s*'([^']+)'/g)].map((m) => m[1])
	const extra = ['/shares/new', '/storage/create', '/health/disk', '/settings/network/wizard', '/help/proxy']
	return [...new Set([...fromNav, ...extra])]
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

	// Detail pages of things that exist: the first pool.
	const pages = pagesToVisit()
	// Asked from the page: the session cookie is Secure, which Playwright's
	// own request client won't send over http, while the browser does on
	// 127.0.0.1.
	const pools = await page.evaluate(() => fetch('/api/v1/pools').then((r) => (r.ok ? r.json() : [])).catch(() => []))
	const pool = Array.isArray(pools) ? pools[0] : undefined
	if (pool?.id || pool?.uuid) pages.push(`/storage/${encodeURIComponent(pool.id || pool.uuid)}`)

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
