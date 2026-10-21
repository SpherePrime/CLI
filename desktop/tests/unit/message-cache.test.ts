import { expect, it } from 'vitest'
import { MessageCache } from '../../src/renderer/src/features/chat/messageCache'
import type { Message } from '../../src/shared/contracts'

const messages = (text: string): Message[] => [
  {
    id: text.slice(0, 2),
    session_id: 's',
    role: 'assistant',
    parts: [{ type: 'text', data: { text } }]
  }
]

it('keeps recently visited chats and evicts the least recently used history', () => {
  const cache = new MessageCache(2, 100000)
  const first = messages('first')
  cache.set('a', first)
  cache.set('b', messages('second'))
  expect(cache.get('a')).toBe(first)
  cache.set('c', messages('third'))
  expect(cache.get('b')).toBeUndefined()
  expect(cache.get('a')).toBe(first)
})

it('bounds retained history by bytes even when there are few image-heavy chats', () => {
  const cache = new MessageCache(8, 4096)
  for (let index = 0; index < 50; index++) cache.set(String(index), messages('a'.repeat(900)))
  expect(cache.size).toBeLessThanOrEqual(2)
  expect(cache.bytes).toBeLessThanOrEqual(4096)
  cache.set('large', messages('a'.repeat(5000)))
  expect(cache.get('large')).toBeUndefined()
  cache.delete('49')
  expect(cache.get('49')).toBeUndefined()
})

it('replaces streamed snapshots without charging unchanged messages repeatedly', () => {
  const cache = new MessageCache(8, 100000)
  const first = messages('small')
  cache.set('a', first)
  const bytes = cache.bytes
  cache.set('a', first)
  expect(cache.bytes).toBe(bytes)
  cache.set('a', [...first, ...messages('more')])
  expect(cache.bytes).toBe(bytes * 2 - 2)
  cache.delete('a')
  expect(cache.bytes).toBe(0)
})
