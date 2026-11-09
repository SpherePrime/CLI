import { describe, expect, it } from 'vitest'
import {
  isOverlayCovering,
  rectsOverlap,
  type OverlayQueryRoot
} from '../../src/renderer/src/features/browser/overlayCoverage'

describe('overlayCoverage', () => {
  const browserRect = { x: 800, y: 100, width: 400, height: 600 }

  it('detects disjoint rects as non-overlapping', () => {
    const popoverOnLeft = { left: 200, top: 200, right: 500, bottom: 500, width: 300, height: 300 }
    expect(rectsOverlap(browserRect, popoverOnLeft)).toBe(false)
  })

  it('detects intersecting rects as overlapping', () => {
    const intersectingPopover = { left: 750, top: 200, right: 900, bottom: 400, width: 150, height: 200 }
    expect(rectsOverlap(browserRect, intersectingPopover)).toBe(true)
  })

  it('ignores zero-sized rects', () => {
    const emptyBox = { left: 850, top: 200, right: 850, bottom: 200, width: 0, height: 0 }
    expect(rectsOverlap(browserRect, emptyBox)).toBe(false)
  })

  it('returns true when a full screen modal dialog is open', () => {
    const root: OverlayQueryRoot = {
      querySelector: (selector: string) => (selector.includes('dialog[open]') ? {} : null),
      querySelectorAll: () => []
    }
    expect(isOverlayCovering(browserRect, root)).toBe(true)
  })

  it('returns false when popover does not overlap browser rect', () => {
    const root: OverlayQueryRoot = {
      querySelector: () => null,
      querySelectorAll: () => [
        {
          getBoundingClientRect: () => ({
            left: 100,
            top: 100,
            right: 400,
            bottom: 400,
            width: 300,
            height: 300
          })
        }
      ]
    }
    expect(isOverlayCovering(browserRect, root)).toBe(false)
  })

  it('returns true when popover overlaps browser rect', () => {
    const root: OverlayQueryRoot = {
      querySelector: () => null,
      querySelectorAll: () => [
        {
          getBoundingClientRect: () => ({
            left: 700,
            top: 100,
            right: 900,
            bottom: 300,
            width: 200,
            height: 200
          })
        }
      ]
    }
    expect(isOverlayCovering(browserRect, root)).toBe(true)
  })

  it('returns false when query root has no overlays', () => {
    const root: OverlayQueryRoot = {
      querySelector: () => null,
      querySelectorAll: () => []
    }
    expect(isOverlayCovering(browserRect, root)).toBe(false)
  })
})
