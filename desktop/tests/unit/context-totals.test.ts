import { describe, expect, it } from 'vitest'
import { contextTotals } from '../../src/renderer/src/features/models/ContextPopover'

const breakdown = {
  categories: [
    { key: 'system_prompt', tokens: 3000, items: 1 },
    { key: 'user_messages', tokens: 1000, items: 2 }
  ],
  estimated_total: 4000,
  reported_total: 8000,
  context_window: 100000,
  auto_summarize_percent: 80,
  estimated_usage: false
}

describe('contextTotals', () => {
  it('prefers the reported total for the meter and header', () => {
    const totals = contextTotals(breakdown, 50000)
    expect(totals.total).toBe(8000)
    expect(totals.capacity).toBe(100000)
    expect(Math.round(totals.percentOfTotal(totals.total))).toBe(8)
  })

  it('falls back to the estimate when the provider reported nothing', () => {
    const totals = contextTotals({ ...breakdown, reported_total: 0 }, 50000)
    expect(totals.total).toBe(4000)
  })

  it('uses the fallback window when no breakdown exists', () => {
    const totals = contextTotals(null, 64000)
    expect(totals.capacity).toBe(64000)
    expect(totals.total).toBe(0)
    expect(totals.percentOfTotal(32000)).toBe(50)
  })

  it('keeps category shares unscaled so percentages sum to 100%', () => {
    const totals = contextTotals(breakdown, 50000)
    const sum = breakdown.categories.reduce(
      (total, category) => total + totals.shareOfEstimate(category),
      0
    )
    expect(sum).toBeCloseTo(1)
    expect(totals.shareOfEstimate(breakdown.categories[0]) * 100).toBeCloseTo(75)
  })

  it('computes free space from the reported total', () => {
    const totals = contextTotals(breakdown, 50000)
    expect(totals.free).toBe(92000)
  })

  it('exposes the estimated-usage flag', () => {
    expect(contextTotals({ ...breakdown, estimated_usage: true }, 50000).estimatedUsage).toBe(true)
  })
})
