import { describe, expect, it } from 'vitest'
import { conversationTiming } from '../../src/renderer/src/features/chat/conversationTiming'
import type { Message } from '../../src/shared/contracts'

function message(
  id: string,
  role: string,
  created_at: number,
  parts: { type: string; data: any }[]
): Message {
  return { id, session_id: 'session', role, created_at, parts }
}

const text = (value: string, hidden = false) => ({
  type: 'text',
  data: hidden ? { text: value, hidden: true } : { text: value }
})
const finish = (reason: string, time: number) => ({ type: 'finish', data: { reason, time } })

describe('conversationTiming', () => {
  it('measures a run from the prompt that started it', () => {
    const timing = conversationTiming([
      message('u1', 'user', 100, [text('start')]),
      message('a1', 'assistant', 101, [finish('tool_use', 105)]),
      message('a2', 'assistant', 106, [finish('end_turn', 117)])
    ])
    expect(timing.startedAt).toBe(100_000)
    expect(timing.completed.get('a2')).toBe(17)
  })

  it('keeps counting when a background job wake-up continues the request', () => {
    const timing = conversationTiming([
      message('u1', 'user', 100, [text('start')]),
      message('a1', 'assistant', 110, [finish('end_turn', 110)]),
      message('u2', 'user', 400, [text('Background job finished', true)]),
      message('a2', 'assistant', 401, [finish('end_turn', 425)])
    ])
    expect(timing.startedAt).toBe(100_000)
    expect(timing.completed.get('a1')).toBe(10)
    expect(timing.completed.get('a2')).toBe(325)
  })

  it('restarts at a new prompt from the user', () => {
    const timing = conversationTiming([
      message('u1', 'user', 100, [text('first')]),
      message('a1', 'assistant', 110, [finish('end_turn', 130)]),
      message('u2', 'user', 500, [text('second')]),
      message('a2', 'assistant', 501, [finish('tool_use', 505)])
    ])
    expect(timing.startedAt).toBe(500_000)
  })

  it('ignores the shell prompts the composer sends', () => {
    const timing = conversationTiming([
      message('u1', 'user', 100, [text('start')]),
      message('u2', 'user', 200, [{ type: 'shell_command', data: { command: 'ls' } }]),
      message('a1', 'assistant', 201, [finish('tool_use', 205)])
    ])
    expect(timing.startedAt).toBe(100_000)
  })
})
