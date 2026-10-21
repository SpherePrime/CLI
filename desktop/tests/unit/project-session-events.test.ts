import { afterEach, expect, it, vi } from 'vitest'
import { watchProjectSessions } from '../../src/renderer/src/features/projects/projectSessionEvents'

afterEach(() => vi.useRealTimers())

it('detects a run from message events without polling an already running stream', async () => {
  vi.useFakeTimers()
  let listener!: (event: any) => void
  let busy = false
  const call = vi.fn(async () => [{ id: 'chat', title: 'Chat', is_busy: busy }])
  const snapshots = vi.fn()
  const stop = watchProjectSessions(
    ['project'],
    {
      call,
      onEvent: (handler) => {
        listener = handler
        return () => {}
      }
    },
    snapshots
  )
  await vi.runAllTimersAsync()
  busy = true
  const message = {
    workspaceId: 'project',
    type: 'message',
    payload: { data: { session_id: 'chat' } }
  }
  listener(message)
  await vi.runAllTimersAsync()
  expect(snapshots).toHaveBeenLastCalledWith('project', [
    { id: 'chat', title: 'Chat', is_busy: true }
  ])
  const reads = call.mock.calls.length
  listener(message)
  await vi.runAllTimersAsync()
  expect(call).toHaveBeenCalledTimes(reads)
  stop()
})

it('refreshes only the project that changes and unsubscribes cleanly', async () => {
  vi.useFakeTimers()
  let listener!: (event: any) => void
  let sessions = [{ id: 'chat', title: 'Chat', is_busy: false }]
  const call = vi.fn(async () => sessions)
  const unsubscribe = vi.fn()
  const onSnapshot = vi.fn()
  const stop = watchProjectSessions(
    ['background'],
    {
      call,
      onEvent: (handler) => {
        listener = handler
        return unsubscribe
      }
    },
    onSnapshot
  )
  await vi.runAllTimersAsync()
  expect(onSnapshot).toHaveBeenLastCalledWith('background', sessions)
  sessions = [{ ...sessions[0], is_busy: true }]
  listener({ workspaceId: 'background', type: 'agent_event', payload: { data: {} } })
  listener({ workspaceId: 'other', type: 'agent_event', payload: { data: {} } })
  await vi.runAllTimersAsync()
  expect(call).toHaveBeenCalledTimes(2)
  expect(onSnapshot).toHaveBeenLastCalledWith('background', sessions)
  sessions = [{ ...sessions[0], is_busy: false }]
  listener({
    workspaceId: 'background',
    type: 'run_complete',
    payload: { data: { session_id: 'chat' } }
  })
  await vi.runAllTimersAsync()
  expect(onSnapshot).toHaveBeenLastCalledWith('background', sessions)
  stop()
  expect(unsubscribe).toHaveBeenCalledOnce()
})
