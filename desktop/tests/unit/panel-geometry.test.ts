import { describe, expect, it } from 'vitest'
import { chatCenterShift, panelGeometry } from '../../src/renderer/src/app/panelGeometry'

describe('panel geometry', () => {
  it('keeps both icon rails and a usable conversation in narrow windows', () => {
    const layout = panelGeometry(480, 256, 334, true, true)
    expect(layout.compact).toBe(true)
    expect(layout.sidebar).toBe(48)
    expect(layout.inspector).toBe(48)
    expect(layout.columns).toBe('56px minmax(0, 1fr) 56px')
  })
  it('preserves expanded panels when the available width fits', () => {
    expect(panelGeometry(1440, 256, 334, true, true).compact).toBe(false)
  })
  it('allows the workspace to use nearly the entire available window', () => {
    const layout = panelGeometry(1440, 256, 1200, true, true)
    expect(layout.inspector).toBe(1200)
    expect(layout.sidebar).toBe(48)
    expect(layout.inspectorMax).toBe(1254)
  })
  it('does not reintroduce manually hidden panels', () => {
    expect(panelGeometry(480, 256, 334, false, false).columns).toBe('minmax(0, 1fr)')
  })
})

it('shifts the chat column toward the window center', () => {
  const wide = panelGeometry(1440, 256, 350, true, true)
  expect(chatCenterShift(wide, true, true, false)).toBe(47)
  expect(chatCenterShift(wide, true, false, false)).toBe(-132)
  expect(chatCenterShift(wide, true, true, true)).toBe(0)
  const compact = panelGeometry(800, 256, 350, true, true)
  expect(chatCenterShift(compact, true, true, false)).toBe(0)
})
