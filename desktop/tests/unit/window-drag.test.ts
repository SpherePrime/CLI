import { describe, expect, it } from 'vitest'
import { shouldIgnoreNativeDrag } from '../../src/renderer/src/host/windowDrag'

function createMockElement(matchingSelectors: string[] = []): Element {
  return {
    closest: (selector: string) => {
      const selectors = selector.split(',').map((item) => item.trim())
      return selectors.some((item) => matchingSelectors.includes(item)) ? ({} as Element) : null
    }
  } as unknown as Element
}

describe('windowDrag', () => {
  it('ignores drag for non-primary mouse buttons', () => {
    const element = createMockElement([])
    expect(shouldIgnoreNativeDrag(element, 1)).toBe(true)
    expect(shouldIgnoreNativeDrag(element, 2)).toBe(true)
  })

  it('ignores drag when target is null', () => {
    expect(shouldIgnoreNativeDrag(null, 0)).toBe(true)
  })

  it('allows dragging on empty titlebar area', () => {
    const titlebar = createMockElement([])
    expect(shouldIgnoreNativeDrag(titlebar, 0)).toBe(false)
  })

  it('ignores drag when clicking native window controls', () => {
    const button = createMockElement(['button', '.native-window-controls'])
    expect(shouldIgnoreNativeDrag(button, 0)).toBe(true)

    const icon = createMockElement(['.native-window-controls'])
    expect(shouldIgnoreNativeDrag(icon, 0)).toBe(true)
  })

  it('ignores drag when clicking titlebar window space', () => {
    const space = createMockElement(['.titlebar-window-space'])
    expect(shouldIgnoreNativeDrag(space, 0)).toBe(true)
  })

  it('ignores drag when clicking breadcrumbs or interactive elements', () => {
    const breadcrumb = createMockElement(['.breadcrumbs'])
    expect(shouldIgnoreNativeDrag(breadcrumb, 0)).toBe(true)

    const input = createMockElement(['input'])
    expect(shouldIgnoreNativeDrag(input, 0)).toBe(true)
  })
})
