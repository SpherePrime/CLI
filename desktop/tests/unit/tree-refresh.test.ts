import { describe, expect, it } from 'vitest'
import { shouldRefreshTreeOnBusyChange } from '../../src/renderer/src/features/files/treeRefresh'

describe('inspector tree busy transition refresh', () => {
  it('refreshes only when busy transitions from true to false on the same workspace', () => {
    expect(shouldRefreshTreeOnBusyChange(false, false, 'ws-1', 'ws-1')).toBe(false)
    expect(shouldRefreshTreeOnBusyChange(false, true, 'ws-1', 'ws-1')).toBe(false)
    expect(shouldRefreshTreeOnBusyChange(true, true, 'ws-1', 'ws-1')).toBe(false)
    expect(shouldRefreshTreeOnBusyChange(true, false, 'ws-1', 'ws-1')).toBe(true)
    expect(shouldRefreshTreeOnBusyChange(true, false, 'ws-1', 'ws-2')).toBe(false)
  })
})
