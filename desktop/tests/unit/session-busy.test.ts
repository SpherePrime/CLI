import { describe, expect, it } from 'vitest'
import { reconcileObservedBusy, type ObservedBusy } from '../../src/renderer/src/app/sessionBusy'

const snapshot = {
  projectId: 'project-1',
  sessionId: 'session-1',
  isBusy: false
}

const observed: ObservedBusy = {
  projectId: 'project-1',
  sessionId: 'session-1',
  busy: true
}

describe('session busy reconciliation', () => {
  it('drops a stale override when the engine reports idle', () => {
    expect(reconcileObservedBusy(observed, snapshot)).toBeNull()
  })

  it('keeps an override while the engine reports busy', () => {
    expect(reconcileObservedBusy(observed, { ...snapshot, isBusy: true })).toBe(observed)
  })

  it('leaves an override for another project or session untouched', () => {
    expect(
      reconcileObservedBusy(observed, { ...snapshot, projectId: 'project-2' })
    ).toBe(observed)
    expect(
      reconcileObservedBusy(observed, { ...snapshot, sessionId: 'session-2' })
    ).toBe(observed)
  })

  it('keeps a null override null', () => {
    expect(reconcileObservedBusy(null, snapshot)).toBeNull()
  })
})
