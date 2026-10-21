import { expect, it } from 'vitest'
import { reuseUnchanged } from '../../src/renderer/src/features/chat/stableMessages'
import { ActivityTracker } from '../../src/main/prime/activity'
import type { Message } from '../../src/shared/contracts'

const message = (id: string, text: string): Message => ({
  id,
  session_id: 's',
  role: 'assistant',
  parts: [{ type: 'text', data: { text } }]
})

it('keeps unchanged message objects between snapshots', () => {
  const previous = [message('a', 'one'), message('b', 'two')]
  const same = reuseUnchanged(previous, [message('a', 'one'), message('b', 'two')])
  expect(same).toBe(previous)
  const updated = reuseUnchanged(previous, [message('a', 'one'), message('b', 'three')])
  expect(updated).not.toBe(previous)
  expect(updated[0]).toBe(previous[0])
  expect(updated[1]).not.toBe(previous[1])
  expect(reuseUnchanged(previous, [message('a', 'one')])).toEqual([previous[0]])
})

it('bounds finished run history', () => {
  const tracker = new ActivityTracker()
  for (let index = 0; index < 20000; index++) tracker.remove('w', `run-${index}`)
  expect((tracker as any).terminal.size).toBeLessThanOrEqual(5000)
  tracker.accept('w', 'run-19999', 's')
  expect(tracker.isBusy()).toBe(false)
})
