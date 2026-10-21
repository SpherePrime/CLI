import { expect, it } from 'vitest'
import type { Message } from '../../src/shared/contracts'
import { conversationTiming } from '../../src/renderer/src/features/chat/conversationTiming'

const message = (
  id: string,
  role: string,
  created_at: number,
  reason?: string,
  time?: number
): Message => ({
  id,
  role,
  session_id: 's',
  created_at,
  parts: reason ? [{ type: 'finish', data: { reason, time } }] : []
})

it('restores the active start across tool steps and steering messages', () => {
  const messages = [
    message('u', 'user', 100),
    message('a', 'assistant', 102, 'tool_use', 110),
    message('steer', 'user', 115),
    message('tool', 'tool', 118),
    message('next', 'assistant', 120)
  ]
  expect(conversationTiming(messages).startedAt).toBe(100000)
  expect(conversationTiming(JSON.parse(JSON.stringify(messages))).startedAt).toBe(100000)
})

it('keeps completed durations from stored finish times and starts the next task separately', () => {
  const timing = conversationTiming([
    message('u1', 'user', 100),
    message('a1', 'assistant', 104, 'tool_use', 110),
    message('a2', 'assistant', 120, 'end_turn', 175),
    message('u2', 'user', 300),
    message('a3', 'assistant', 305, 'canceled', 312),
    message('u3', 'user', 500)
  ])
  expect(timing.startedAt).toBe(500000)
  expect([...timing.completed]).toEqual([
    ['a2', 75],
    ['a3', 12]
  ])
})

it('does not invent a clock for history without timestamps', () => {
  expect(
    conversationTiming([{ id: 'a', role: 'assistant', session_id: 's', parts: [] }]).startedAt
  ).toBeNull()
})

it('starts the task after a standalone shell command without counting the idle pause', () => {
  const shell = {
    ...message('shell', 'user', 100),
    parts: [{ type: 'shell_command', data: { command: 'pwd', output: '/project', exit_code: 0 } }]
  }
  const active = [shell, message('u', 'user', 7200), message('a', 'assistant', 7201)]
  expect(conversationTiming(active).startedAt).toBe(7200000)
  expect(
    conversationTiming([
      ...active,
      message('done', 'assistant', 7205, 'end_turn', 7212)
    ]).completed.get('done')
  ).toBe(12)
})
