import { expect, it } from 'vitest'
import { ActivityTracker } from '../../src/main/prime/activity'
it('does not treat default SSE is_busy false as authoritative', () => {
  const activity = new ActivityTracker()
  activity.accept('a', 'run-1', 'session')
  activity.event('a', 'session', { id: 'session', is_busy: false })
  expect(activity.isBusy()).toBe(true)
})
it('keeps queued runs active after the preceding run completes', () => {
  const activity = new ActivityTracker()
  activity.accept('a', 'run-1', 'session')
  activity.accept('a', 'run-2', 'session')
  activity.event('a', 'run_complete', { run_id: 'run-1', session_id: 'session' })
  expect(activity.isBusy()).toBe(true)
  activity.event('a', 'run_complete', { run_id: 'run-2', session_id: 'session' })
  expect(activity.isBusy()).toBe(false)
})
it('does not resurrect a completed run from an older snapshot', () => {
  const activity = new ActivityTracker()
  activity.accept('a', 'run-1', 'session')
  activity.event('a', 'run_complete', { run_id: 'run-1', session_id: 'session' })
  activity.reconcile('a', [{ run_id: 'run-1', session_id: 'session', status: 'running' }])
  expect(activity.isBusy()).toBe(false)
})
