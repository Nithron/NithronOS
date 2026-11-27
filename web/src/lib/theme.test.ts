import { afterEach, describe, expect, it } from 'vitest'
import { applyTheme, isThemeChoice, THEME_STORAGE_KEY } from './theme'

// Node's own (flag-gated) localStorage hides jsdom's: give the page one.
const store = new Map<string, string>()
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, String(v)),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
  },
})

describe('applyTheme', () => {
  afterEach(() => {
    document.documentElement.removeAttribute('data-theme')
    window.localStorage.clear()
  })

  it('pins a light or dark theme on <html> and remembers it', () => {
    applyTheme('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('follows the system again: no data-theme, nothing stored', () => {
    applyTheme('light')
    applyTheme('auto')
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBeNull()
  })
})

describe('isThemeChoice', () => {
  it('accepts the API values only', () => {
    expect(['auto', 'light', 'dark'].every(isThemeChoice)).toBe(true)
    expect(isThemeChoice('system')).toBe(false)
    expect(isThemeChoice(undefined)).toBe(false)
  })
})
