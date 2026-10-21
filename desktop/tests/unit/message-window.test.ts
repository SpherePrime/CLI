import { expect, it } from 'vitest'
import {
  mergeMessagePage,
  updateHistoryMessage
} from '../../src/renderer/src/features/chat/messageWindow'
import type { Message } from '../../src/shared/contracts'
const messages = (start: number, end: number): Message[] =>
  Array.from({ length: end - start }, (_, i) => ({
    id: String(start + i),
    session_id: 's',
    role: 'user',
    parts: [],
    created_at: start + i
  }))
const page = (start: number, end: number) => ({
  messages: messages(start, end),
  has_before: start > 0,
  has_after: end < 300,
  timing: { completed: {} }
})
it('pages backwards and forwards with a bounded reversible window', () => {
  let window = mergeMessagePage(undefined, page(240, 300), 'latest')
  window = mergeMessagePage(window, page(180, 240), 'before')
  window = mergeMessagePage(window, page(120, 180), 'before')
  window = mergeMessagePage(window, page(60, 120), 'before')
  expect(window.messages.map((item) => item.id)).toEqual(messages(60, 240).map((item) => item.id))
  expect(window.has_after).toBe(true)
  window = mergeMessagePage(window, page(240, 300), 'after')
  expect(window.messages.map((item) => item.id)).toEqual(messages(120, 300).map((item) => item.id))
  expect(window.has_before).toBe(true)
})
it('refresh and live events preserve the historical viewport without injecting a gap', () => {
  const window = mergeMessagePage(undefined, page(60, 120), 'latest')
  const refreshed = mergeMessagePage(window, page(240, 300), 'latest')
  expect(refreshed.messages).toEqual(window.messages)
  expect(updateHistoryMessage(refreshed, messages(300, 301)[0], 'created').messages).toEqual(
    window.messages
  )
})

it('marks a missed tail as pageable and ignores events older than the loaded window', () => {
  const recent = mergeMessagePage(undefined, { ...page(120, 180), has_after: false }, 'latest')
  const refreshed = mergeMessagePage(recent, page(240, 300), 'latest')
  expect(refreshed.messages).toEqual(recent.messages)
  expect(refreshed.has_after).toBe(true)
  expect(updateHistoryMessage(recent, messages(10, 11)[0], 'created').messages).toEqual(
    recent.messages
  )
})

it('finishes a task using the persisted start outside the loaded window', () => {
  const recent = mergeMessagePage(
    undefined,
    {
      ...page(120, 180),
      has_after: false,
      timing: { started_at: 100, completed: {} }
    },
    'latest'
  )
  const finished = updateHistoryMessage(
    recent,
    {
      id: '180',
      session_id: 's',
      role: 'assistant',
      created_at: 190,
      parts: [{ type: 'finish', data: { reason: 'end_turn', time: 250 } }]
    },
    'created'
  )
  expect(finished.timing.completed['180']).toBe(150)
  expect(finished.timing.started_at).toBeUndefined()
})
