import { describe, expect, it } from 'vitest'
import { panelGeometry } from '../../src/renderer/src/app/panelGeometry'

describe('panel geometry', () => {
  it('keeps both icon rails and a usable conversation in narrow windows', () => {
    const layout = panelGeometry(480, 256, 334, true, true)
    expect(layout.compact).toBe(true)
    expect(layout.sidebar).toBe(48)
    expect(layout.inspector).toBe(48)
    expect(layout.columns).toBe('48px minmax(0, 1fr) 48px')
  })
  it('preserves expanded panels when the available width fits', () => {
    expect(panelGeometry(1440, 256, 334, true, true).compact).toBe(false)
  })
  it('allows the workspace to use nearly the entire available window', () => {
    const layout = panelGeometry(1440, 256, 1200, true, true)
    expect(layout.inspector).toBe(1200)
    expect(layout.sidebar).toBe(48)
    expect(layout.inspectorMax).toBe(1270)
  })
  it('does not reintroduce manually hidden panels', () => {
    expect(panelGeometry(480, 256, 334, false, false).columns).toBe('minmax(0, 1fr)')
  })
})
