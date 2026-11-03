import { describe, expect, it } from 'vitest'
import type { Message } from '../../src/shared/contracts'
import {
  updateHistoryMessage,
  type MessagePage
} from '../../src/renderer/src/features/chat/messageWindow'

const prompt = (id: string, created_at: number, hidden = false): Message => ({
  id,
  session_id: 'session',
  role: 'user',
  created_at,
  parts: [{ type: 'text', data: { text: id, hidden } }]
})
const answer = (id: string, created_at: number, time: number, reason = 'end_turn'): Message => ({
  id,
  session_id: 'session',
  role: 'assistant',
  created_at,
  parts: [{ type: 'finish', data: { time, reason } }]
})
const previousTask = (): MessagePage => ({
  messages: [prompt('u1', 100), answer('a1', 101, 110)],
  has_before: false,
  has_after: false,
  timing: { started_at: 100, completed: { a1: 10 } }
})

describe('history timing during live updates', () => {
  it('starts a new task after hours of inactivity in a loaded chat', () => {
    const next = updateHistoryMessage(previousTask(), prompt('u2', 14_500), 'created')
    expect(next.timing.started_at).toBe(14_500)
    const finished = updateHistoryMessage(next, answer('a2', 14_501, 14_520), 'created')
    expect(finished.timing.completed).toEqual({ a1: 10, a2: 20 })
  })

  it('retains the task start through tool calls and generated continuations', () => {
    let page = updateHistoryMessage(previousTask(), prompt('u2', 500), 'created')
    page = updateHistoryMessage(page, answer('tool', 501, 505, 'tool_use'), 'created')
    page = updateHistoryMessage(page, answer('a2', 506, 510), 'created')
    page = updateHistoryMessage(page, prompt('wake', 520, true), 'created')
    page = updateHistoryMessage(page, answer('a3', 521, 525), 'created')
    expect(page.timing.started_at).toBe(500)
    expect(page.timing.completed).toEqual({ a1: 10, a2: 10, a3: 25 })
  })

  it('does not start a task for a shell command', () => {
    const shell: Message = {
      ...prompt('shell', 500),
      parts: [{ type: 'shell_command', data: { command: 'ls' } }]
    }
    const page = updateHistoryMessage(previousTask(), shell, 'created')
    expect(page.timing.started_at).toBe(100)
  })
})
