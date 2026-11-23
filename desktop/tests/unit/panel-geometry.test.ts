import { describe, expect, it } from 'vitest'
import { panelGeometry } from '../../src/renderer/src/app/panelGeometry'
import {
  activeWorkerCount,
  formatWorkerDuration,
  workerRoleKind
} from '../../src/renderer/src/features/agents/agentPresentation'
import type { AgentWorker } from '../../src/shared/agents'

describe('panelGeometry inspector bounds', () => {
  it('keeps at least 460px for the main chat when inspector is maximized without sidebar', () => {
    const layout = panelGeometry(1440, 256, 1200, true, true, true, false, false)
    expect(layout.inspectorMax).toBe(860)
    expect(layout.inspector).toBe(860)
    const remainingChat = 1440 - (48 + 8) * 2 - 8 - layout.inspector
    expect(remainingChat).toBeGreaterThanOrEqual(460)
  })

  it('bounds inspector on 1280px viewports without collapsing chat', () => {
    const layout = panelGeometry(1280, 256, 1100, true, true, true, false, false)
    expect(layout.inspector).toBe(700)
    const remainingChat = 1280 - (48 + 8) * 2 - 8 - layout.inspector
    expect(remainingChat).toBeGreaterThanOrEqual(460)
  })
})

describe('agentPresentation helpers', () => {
  it('maps agent roles and formats durations cleanly', () => {
    expect(workerRoleKind('code')).toBe('code')
    expect(workerRoleKind('Explore')).toBe('explore')
    expect(workerRoleKind('verifier')).toBe('review')
    expect(workerRoleKind('plan')).toBe('plan')
    expect(workerRoleKind('custom')).toBe('general')

    const now = 1_700_000_000
    expect(formatWorkerDuration(now - 45, now, true, 'ru', now)).toBe('45с')
    expect(formatWorkerDuration(now - 135, now, true, 'ru', now)).toBe('2м 15с')
    expect(formatWorkerDuration(now - 180, now - 60, false, 'ru', now)).toBe('2м')
    expect(formatWorkerDuration(now - 135, now, true, 'en', now)).toBe('2m 15s')
  })

  it('counts active workers accurately', () => {
    const workers = [
      { id: '1', agent: 'code', status: 'running' },
      { id: '2', agent: 'explore', status: 'queued' },
      { id: '3', agent: 'review', status: 'done' }
    ] as AgentWorker[]
    expect(activeWorkerCount(workers)).toBe(2)
  })
})
