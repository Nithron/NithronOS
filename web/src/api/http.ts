import { toast } from '@/components/ui/toast'

export class ErrProxyMisconfigured extends Error {
  status: number
  contentType: string
  snippet: string
  constructor(message: string, status: number, contentType: string, snippet: string) {
    super(message)
    this.name = 'ErrProxyMisconfigured'
    this.status = status
    this.contentType = contentType
    this.snippet = snippet
  }
}

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...(init || {}),
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...((init && init.headers) || {}),
    },
  })
  const ct = res.headers.get('content-type') || ''
  // If not JSON at all, raise proxy misconfig, regardless of status
  if (!ct.includes('application/json')) {
    let snippet = ''
    try {
      const text = await res.text()
      snippet = text.slice(0, 200)
    } catch {}
    throw new ErrProxyMisconfigured('Response is not JSON', res.status, ct, snippet)
  }
  if (!res.ok) {
    // Parse JSON error body similar to request()
    let message = ''
    try {
      const body: any = await res.json()
      const err = body?.error
      if (err) {
        message = String(err.message || '')
      }
    } catch {}
    const msg = message ? `HTTP ${res.status}: ${message}` : `HTTP ${res.status}`
    const error: any = new Error(msg)
    error.status = res.status
    throw error
  }
  return (await res.json()) as T
}

export async function apiGet<T>(path: string, init?: RequestInit): Promise<T> {
	return request<T>(path, { method: 'GET', ...(init || {}) })
}

export async function apiPost<T>(path: string, body?: any): Promise<T> {
	return request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
}

export async function apiPatch<T>(path: string, body?: any): Promise<T> {
	return request<T>(path, { method: 'PATCH', body: body ? JSON.stringify(body) : undefined })
}

export async function apiDelete<T = void>(path: string): Promise<T> {
	return request<T>(path, { method: 'DELETE' })
}

function getCSRFCookie(): string | null {
	const m = document.cookie.match(/(?:^|; )nos_csrf=([^;]*)/)
	return m ? decodeURIComponent(m[1]) : null
}

export async function apiPostAuth<T>(path: string, token: string, body?: any): Promise<T> {
	return request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined, headers: { Authorization: `Bearer ${token}` } })
}

// One refresh at a time: concurrent 401s wait for the same refresh. (Each
// refresh replaces the refresh token, so parallel refreshes would present a
// spent token. This also called /api/auth/refresh, which doesn't exist, so
// users were sent to the login page when the 15-minute access cookie expired.)
let refreshing: Promise<boolean> | null = null

export function refreshSession(): Promise<boolean> {
	if (!refreshing) {
		refreshing = fetch('/api/v1/auth/refresh', { method: 'POST', credentials: 'include' })
			.then((r) => r.ok)
			.catch(() => false)
			.finally(() => {
				refreshing = null
			})
	}
	return refreshing
}

async function request<T>(path: string, init: RequestInit, retried = false): Promise<T> {
	const isSetup = path.startsWith('/api/v1/setup/')
	const csrf = getCSRFCookie()
	const res = await fetch(path, {
		...init,
		credentials: 'include',
		headers: {
			'Accept': 'application/json',
			...(init.body ? { 'Content-Type': 'application/json' } : {}),
			...(csrf ? { 'X-CSRF-Token': csrf } : {}),
			...(init.headers || {}),
		},
	})
	// Handle setup 410 gracefully: let callers treat as non-firstBoot
	if (isSetup && res.status === 410) {
		toast.error('Setup already completed. You can sign in.')
		throw new Error('HTTP 410')
	}
	if (res.status === 401 && !retried) {
		// Try one refresh, then retry the request.
		if (await refreshSession()) {
			return request<T>(path, init, true)
		}
		// refresh failed — log out
		window.location.href = '/login'
	}
	if (!res.ok) {
		const ct = res.headers.get('content-type') || ''
		let message = ''
		let retryAfterSec = 0
		let code = ''
		let body: any = undefined
		try {
			if (ct.includes('application/json')) {
				body = await res.json()
				const err = (body as any)?.error
				if (err) {
					message = String(err.message || '')
					retryAfterSec = Number(err.retryAfterSec || 0)
					code = String(err.code || '')
				}
			} else {
				message = await res.text()
			}
		} catch {}
		// Global toasts for common statuses and typed codes
		if (code === 'setup.write_failed') {
			toast.error('Service cannot write /etc/nos/users.json. See Admin → Logs.')
		} else if (code === 'setup.otp.expired') {
			toast.error('Your setup code expired. Request a new one.')
		} else if (code === 'setup.otp.invalid') {
			toast.error('Invalid setup code. Check and try again.')
		} else if (code === 'setup.session.invalid') {
			toast.error('Setup session invalid. Restart setup from the beginning.')
		} else if (code === 'auth.csrf.missing' || code === 'auth.csrf.invalid') {
			toast.error('Your session expired. Please sign in again.')
		} else if (res.status === 429) {
			const ra = retryAfterSec || parseInt(res.headers.get('Retry-After') || '0', 10) || 0
			toast.error(ra > 0 ? `Rate limited. Try again in ${ra}s` : 'Rate limited. Please try again shortly.')
		} else if (res.status === 423) {
			toast.error('Account temporarily locked. Please try again later.')
		} else if (res.status >= 500) {
			toast.error(message || `Request failed (${res.status})`)
		}
		const msg = message ? `HTTP ${res.status}: ${message}` : `HTTP ${res.status}`
		const error: any = new Error(msg)
		error.status = res.status
		if (body !== undefined) error.data = body
		throw error
	}
	if (res.status === 204) return undefined as unknown as T
	return (await res.json()) as T
}


