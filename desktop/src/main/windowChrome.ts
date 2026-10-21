import { nativeTheme, type BrowserWindow } from 'electron'
import type { Preferences } from '../shared/contracts'

const backdrop = { color: [3, 6, 11], opacity: 0.6 }

function hex(color: string) {
  return [1, 3, 5].map((index) => parseInt(color.slice(index, index + 2), 16))
}

export function dimColor(color: string) {
  return (
    '#' +
    hex(color)
      .map((value, index) =>
        Math.round(value * (1 - backdrop.opacity) + backdrop.color[index] * backdrop.opacity)
          .toString(16)
          .padStart(2, '0')
      )
      .join('')
  )
}

export function windowChromeColors(dimmed = false) {
  const colors = nativeTheme.shouldUseDarkColors
    ? { color: '#0a0a0a', symbolColor: '#f2f0e9', height: 44 }
    : { color: '#e7e4dc', symbolColor: '#24221c', height: 44 }
  if (!dimmed) return colors
  return { ...colors, color: dimColor(colors.color), symbolColor: dimColor(colors.symbolColor) }
}

export function applyWindowChrome(
  window: BrowserWindow | null | undefined,
  theme: Preferences['theme'],
  dimmed = false
) {
  if (nativeTheme.themeSource !== theme) nativeTheme.themeSource = theme
  if (!window || window.isDestroyed()) return
  const colors = windowChromeColors(dimmed)
  window.setTitleBarOverlay(colors)
  if (dimmed) return
  window.setBackgroundColor(colors.color)
  if (process.platform === 'win32') window.setAccentColor(colors.color)
}
