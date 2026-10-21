import { expect, it, vi } from 'vitest'

vi.mock('electron', () => ({ nativeTheme: { shouldUseDarkColors: false } }))

it('dims native window controls like the modal backdrop', async () => {
  const { dimColor, windowChromeColors } = await import('../../src/main/windowChrome')
  expect(dimColor('#e7e4dc')).toBe('#5e5f5f')
  expect(windowChromeColors().color).toBe('#e7e4dc')
  expect(windowChromeColors(true).color).toBe('#5e5f5f')
})
