/**
 * The theme: the system's light or dark (the tokens follow
 * prefers-color-scheme), or one pinned by an administrator through the
 * appearance setting. A pinned theme is data-theme on <html>; the choice is
 * also kept in this browser so the sign-in page and the first paint use it
 * (public/theme-init.js reads it before anything is drawn).
 */
export type ThemeChoice = 'auto' | 'light' | 'dark'

export const THEME_STORAGE_KEY = 'nos.theme'

export const themeChoices: ReadonlyArray<{ value: ThemeChoice; label: string }> = [
  { value: 'auto', label: 'Follow the system' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

export function isThemeChoice(v: unknown): v is ThemeChoice {
  return v === 'auto' || v === 'light' || v === 'dark'
}

export function applyTheme(choice: ThemeChoice, root: HTMLElement = document.documentElement): void {
  if (choice === 'auto') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', choice)
  try {
    if (choice === 'auto') window.localStorage.removeItem(THEME_STORAGE_KEY)
    else window.localStorage.setItem(THEME_STORAGE_KEY, choice)
  } catch {
    // Storage unavailable: the theme still applies to this page.
  }
}
