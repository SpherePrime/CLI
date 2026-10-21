import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { MessageCoalescer } from '../../src/main/prime/messageCoalescer'
import { queueMessageEvent } from '../../src/renderer/src/features/chat/messageEvents'
import type { DesktopEvent } from '../../src/shared/contracts'

const update = (id: string, text: string, action = 'updated'): DesktopEvent => ({
  workspaceId: 'w',
  type: 'message',
  payload: { action, data: { id, text } }
})

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

it('emits only the latest streaming update per message', () => {
  const emitted: DesktopEvent[] = []
  const coalescer = new MessageCoalescer((event) => emitted.push(event), 50)
  coalescer.push(update('a', '1'))
  coalescer.push(update('a', '12'))
  coalescer.push(update('b', 'x'))
  coalescer.push(update('a', '123'))
  expect(emitted).toEqual([])
  vi.advanceTimersByTime(50)
  expect(emitted.map((event) => event.payload.data)).toEqual([
    { id: 'a', text: '123' },
    { id: 'b', text: 'x' }
  ])
})

it('flushes pending updates before any other event', () => {
  const emitted: DesktopEvent[] = []
  const coalescer = new MessageCoalescer((event) => emitted.push(event), 50)
  coalescer.push(update('a', 'final'))
  coalescer.push({ workspaceId: 'w', type: 'run_complete', payload: { data: {} } })
  coalescer.push(update('c', 'new', 'created'))
  expect(emitted.map((event) => event.type)).toEqual(['message', 'run_complete', 'message'])
  expect(emitted[0].payload.data.text).toBe('final')
  vi.advanceTimersByTime(100)
  expect(emitted).toHaveLength(3)
})

it('drops pending updates on dispose', () => {
  const emitted: DesktopEvent[] = []
  const coalescer = new MessageCoalescer((event) => emitted.push(event), 50)
  coalescer.push(update('a', '1'))
  coalescer.dispose()
  vi.advanceTimersByTime(100)
  expect(emitted).toEqual([])
})

it('keeps one buffered event per message and preserves creation', () => {
  const queue = new Map<string, { action: string; message: any }>()
  queueMessageEvent(queue, 'a', { action: 'created', message: { id: 'a', text: '' } })
  queueMessageEvent(queue, 'a', { action: 'updated', message: { id: 'a', text: 'x' } })
  queueMessageEvent(queue, 'b', { action: 'updated', message: { id: 'b' } })
  expect([...queue.values()]).toEqual([
    { action: 'created', message: { id: 'a', text: 'x' } },
    { action: 'updated', message: { id: 'b' } }
  ])
  queueMessageEvent(queue, 'a', { action: 'deleted', message: { id: 'a' } })
  expect(queue.get('a')!.action).toBe('deleted')
  queueMessageEvent(queue, 'c', { action: 'updated', message: { id: 'c' } }, 2)
  expect([...queue.keys()]).toEqual(['b', 'c'])
})
