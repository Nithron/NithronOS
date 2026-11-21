import '@testing-library/jest-dom'
import { vi } from 'vitest'

// Global mock for auth to avoid async effects during tests
vi.mock('@/lib/auth', () => ({
	AuthProvider: ({ children }: any) => children,
	AuthGuard: ({ children }: any) => children,
	useAuth: () => ({
		session: null,
		loading: false,
		error: null,
		checkSession: vi.fn(),
		login: vi.fn(),
		logout: vi.fn(),
	}),
}))
vi.mock('../src/lib/auth', () => ({
	AuthProvider: ({ children }: any) => children,
	AuthGuard: ({ children }: any) => children,
	useAuth: () => ({
		session: null,
		loading: false,
		error: null,
		checkSession: vi.fn(),
		login: vi.fn(),
		logout: vi.fn(),
	}),
}))
vi.mock('../lib/auth', () => ({
	AuthProvider: ({ children }: any) => children,
	AuthGuard: ({ children }: any) => children,
	useAuth: () => ({
		session: null,
		loading: false,
		error: null,
		checkSession: vi.fn(),
		login: vi.fn(),
		logout: vi.fn(),
	}),
}))

// Minimal DOM shims for tests
window.alert = window.alert || (()=>{})
window.confirm = window.confirm || (()=>true)

// Silence React Router v7 future flag warnings in test logs
const origWarn = console.warn
console.warn = (...args: any[]) => {
	const msg = String(args[0] || '')
	if (msg.includes('React Router Future Flag Warning')) return
	origWarn(...args)
}

// Filter noisy console.error messages that are expected in tests
const origError = console.error
console.error = (...args: any[]) => {
	const msg = String(args[0] || '')
	if (msg.includes('Setup state check failed') || msg.includes('Session check failed')) return
	origError(...args)
}

// Prevent unhandled promise rejections and errors from failing the run in JSDOM
if (typeof window !== 'undefined' && window.addEventListener) {
	window.addEventListener('unhandledrejection', (event) => {
		event.preventDefault()
	})
	window.addEventListener('error', (event) => {
		event.preventDefault()
	})
}

// Node fallbacks (just in case)
if (typeof process !== 'undefined' && (process as any).on) {
	(process as any).on('unhandledRejection', () => {})
	;(process as any).on('uncaughtException', () => {})
}

