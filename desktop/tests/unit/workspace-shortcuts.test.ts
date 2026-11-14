import { describe, expect, it } from 'vitest'
import { stepZoom } from '../../src/renderer/src/app/useWorkspaceShortcuts'

describe('workspace shortcut zoom rounding', () => {
  it('rounds zoom cleanly to one decimal place without float drift', () => {
    let zoom = 1
    zoom = stepZoom(zoom, 0.1)
    zoom = stepZoom(zoom, 0.1)
    zoom = stepZoom(zoom, 0.1)
    expect(zoom).toBe(1.3)

    zoom = stepZoom(zoom, -0.1)
    zoom = stepZoom(zoom, -0.1)
    expect(zoom).toBe(1.1)
  })

  it('clamps zoom within 0.8 and 2.0', () => {
    expect(stepZoom(0.8, -0.1)).toBe(0.8)
    expect(stepZoom(2, 0.1)).toBe(2)
  })
})
