import { nativeTheme, type BrowserWindow } from 'electron'
import type { Preferences } from '../shared/contracts'

export function windowChromeColors() {
  return nativeTheme.shouldUseDarkColors
    ? { color: '#171716', symbolColor: '#f2f0e9', height: 40 }
    : { color: '#f0eee8', symbolColor: '#24221c', height: 40 }
}

export function applyWindowChrome(
  window: BrowserWindow | null | undefined,
  theme: Preferences['theme']
) {
  if (nativeTheme.themeSource !== theme) nativeTheme.themeSource = theme
  if (!window || window.isDestroyed()) return
  const colors = windowChromeColors()
  window.setTitleBarOverlay(colors)
  window.setBackgroundColor(colors.color)
  if (process.platform === 'win32') window.setAccentColor(colors.color)
}
