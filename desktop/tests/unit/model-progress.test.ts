import { describe, expect, it } from 'vitest'
import { progressPercent } from '../../src/renderer/src/features/settings/modelProgress'

describe('model progress', () => {
  it('keeps preparation indeterminate until a total is known', () => {
    expect(progressPercent(0, 0)).toBeUndefined()
    expect(progressPercent(50, undefined)).toBeUndefined()
    expect(progressPercent(10, Number.NaN)).toBeUndefined()
  })

  it('reports and bounds actual download progress', () => {
    expect(progressPercent(42, 100)).toBe(42)
    expect(progressPercent(120, 100)).toBe(100)
    expect(progressPercent(-2, 100)).toBe(0)
    expect(progressPercent(undefined, 100)).toBeUndefined()
  })
})
