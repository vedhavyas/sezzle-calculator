import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyTheme, nextTheme, readTheme, saveTheme, themeLabel } from './theme'

beforeEach(() => {
  window.localStorage.clear()
  delete document.documentElement.dataset.theme
})

describe('theme', () => {
  it('defaults to the system theme', () => {
    expect(readTheme()).toBe('system')
  })

  it('reads a saved explicit theme', () => {
    saveTheme('dark')
    expect(readTheme()).toBe('dark')
  })

  it('ignores a stored value that is not a theme', () => {
    window.localStorage.setItem('calculator-theme', 'sepia')
    expect(readTheme()).toBe('system')
  })

  it('falls back to the system theme when storage is unavailable', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage is denied')
    })
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage is denied')
    })
    try {
      expect(readTheme()).toBe('system')
      expect(() => saveTheme('dark')).not.toThrow()
    } finally {
      getItem.mockRestore()
      setItem.mockRestore()
    }
  })

  it('cycles system, light and dark', () => {
    expect(nextTheme('system')).toBe('light')
    expect(nextTheme('light')).toBe('dark')
    expect(nextTheme('dark')).toBe('system')
  })

  it('marks the document only for an explicit theme', () => {
    applyTheme('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
    applyTheme('light')
    expect(document.documentElement.dataset.theme).toBe('light')
    applyTheme('system')
    expect(document.documentElement.dataset.theme).toBeUndefined()
  })

  it('labels every theme', () => {
    expect(themeLabel('system')).toBe('System')
    expect(themeLabel('light')).toBe('Light')
    expect(themeLabel('dark')).toBe('Dark')
  })
})
