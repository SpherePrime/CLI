import { describe, expect, it } from 'vitest'
import { chatCenterShift, panelGeometry } from '../../src/renderer/src/app/panelGeometry'

describe('panel geometry', () => {
  it('keeps both icon rails and a usable conversation in narrow windows', () => {
    const layout = panelGeometry(480, 256, 334, true, true)
    expect(layout.compact).toBe(true)
    expect(layout.sidebarOpen).toBe(false)
    expect(layout.inspectorOpen).toBe(false)
    expect(layout.columns).toBe('56px minmax(0, 1fr) 56px')
  })
  it('opens panels next to permanent rails in wide windows', () => {
    const layout = panelGeometry(1440, 256, 334, true, true)
    expect(layout.sidebarOpen).toBe(true)
    expect(layout.inspectorOpen).toBe(true)
    expect(layout.columns).toBe('56px 256px 8px minmax(0, 1fr) 8px 334px 56px')
  })
  it('keeps collapsed panels as resizable icon rails', () => {
    const layout = panelGeometry(1440, 256, 350, true, true, true, true)
    expect(layout.sidebarOpen).toBe(false)
    expect(layout.inspectorOpen).toBe(false)
    expect(layout.columns).toBe('48px 8px minmax(0, 1fr) 8px 48px')
  })
  it('closes the project panel when the workspace leaves too little room', () => {
    const layout = panelGeometry(1440, 256, 1100, true, true)
    expect(layout.inspector).toBe(1100)
    expect(layout.sidebarOpen).toBe(false)
    expect(layout.inspectorMax).toBe(1198)
  })
  it('does not reintroduce manually hidden panels', () => {
    expect(panelGeometry(480, 256, 334, false, false).columns).toBe('minmax(0, 1fr)')
  })
})

it('shifts the chat column toward the window center', () => {
  const wide = panelGeometry(1440, 256, 350, true, true)
  expect(chatCenterShift(wide, true, true, false)).toBe(47)
  expect(chatCenterShift(wide, true, true, true)).toBe(0)
  const collapsed = panelGeometry(1440, 256, 350, true, true, false, true)
  expect(chatCenterShift(collapsed, true, true, false)).toBe(-132)
  const compact = panelGeometry(800, 256, 350, true, true)
  expect(chatCenterShift(compact, true, true, false)).toBe(0)
})
