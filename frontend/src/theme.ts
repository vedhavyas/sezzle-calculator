export type Theme = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'calculator-theme'
const ORDER: Theme[] = ['system', 'light', 'dark']

export function readTheme(): Theme {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY)
    return saved === 'light' || saved === 'dark' ? saved : 'system'
  } catch {
    return 'system'
  }
}

export function saveTheme(theme: Theme) {
  try {
    window.localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // A browser that refuses storage still gets the in-memory choice.
  }
}

export function applyTheme(theme: Theme) {
  if (theme === 'system') delete document.documentElement.dataset.theme
  else document.documentElement.dataset.theme = theme
}

export function nextTheme(theme: Theme): Theme {
  return ORDER[(ORDER.indexOf(theme) + 1) % ORDER.length]
}

export function themeLabel(theme: Theme): string {
  return theme === 'system' ? 'System' : theme === 'light' ? 'Light' : 'Dark'
}
