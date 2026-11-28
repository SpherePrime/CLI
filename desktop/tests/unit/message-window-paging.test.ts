import { describe, expect, it } from 'vitest'
import type { Message } from '../../src/shared/contracts'
import {
  historyWindowLimit,
  mergeMessagePage,
  updateHistoryMessage,
  type MessagePage
} from '../../src/renderer/src/features/chat/messageWindow'

function userMessage(id: string, createdAt: number, text = id): Message {
  return {
    id,
    session_id: 's1',
    role: 'user',
    created_at: createdAt,
    parts: [{ type: 'text', data: { text } }]
  }
}

function assistantToolCall(id: string, callId: string, createdAt: number): Message {
  return {
    id,
    session_id: 's1',
    role: 'assistant',
    created_at: createdAt,
    parts: [
      {
        type: 'tool_call',
        data: { id: callId, name: 'view', input: '{"file_path":"src/app.ts"}' }
      },
      { type: 'finish', data: { reason: 'tool_use', time: createdAt } }
    ]
  }
}

function toolResult(id: string, callId: string, createdAt: number): Message {
  return {
    id,
    session_id: 's1',
    role: 'tool',
    created_at: createdAt,
    parts: [
      {
        type: 'tool_result',
        data: { tool_call_id: callId, name: 'view', content: 'ok' }
      }
    ]
  }
}

function assistantText(id: string, text: string, createdAt: number): Message {
  return {
    id,
    session_id: 's1',
    role: 'assistant',
    created_at: createdAt,
    parts: [
      { type: 'text', data: { text } },
      { type: 'finish', data: { reason: 'end_turn', time: createdAt } }
    ]
  }
}

function buildLongFirstTask(toolSteps: number): Message[] {
  const messages: Message[] = [userMessage('u1', 100, 'Do the first task')]
  let ts = 101
  for (let i = 1; i <= toolSteps; i++) {
    messages.push(assistantToolCall(`a-${i}`, `call-${i}`, ts++))
    messages.push(toolResult(`t-${i}`, `call-${i}`, ts++))
  }
  messages.push(assistantText('a-final', 'Final response text', ts++))
  return messages
}

describe('messageWindow pagination and tool-heavy turns', () => {
  it('keeps the full first task including final text during live updates beyond 180 raw tool rows', () => {
    const all = buildLongFirstTask(105)
    let page: MessagePage = {
      messages: [],
      has_before: false,
      has_after: false,
      timing: { completed: {} }
    }
    for (const msg of all) {
      page = updateHistoryMessage(page, msg, 'created')
    }
    expect(page.messages).toHaveLength(all.length)
    expect(page.messages[0].id).toBe('u1')
    expect(page.messages.at(-1)?.id).toBe('a-final')
    expect(page.has_before).toBe(false)
    expect(page.has_after).toBe(false)
  })

  it('does not evict final assistant text when paging before across a long tool-heavy task', () => {
    const all = buildLongFirstTask(110)
    const sliceBefore = (beforeId: string, limit: number): MessagePage => {
      const end = all.findIndex((item) => item.id === beforeId)
      const start = Math.max(0, end - limit)
      return {
        messages: all.slice(start, end),
        has_before: start > 0,
        has_after: end < all.length,
        timing: { completed: {} }
      }
    }

    let windowPage = mergeMessagePage(
      undefined,
      {
        messages: all.slice(-60),
        has_before: true,
        has_after: false,
        timing: { completed: {} }
      },
      'latest'
    )

    while (windowPage.has_before) {
      const older = sliceBefore(windowPage.messages[0].id, 60)
      windowPage = mergeMessagePage(windowPage, older, 'before')
    }

    expect(windowPage.messages[0].id).toBe('u1')
    expect(windowPage.messages.at(-1)?.id).toBe('a-final')
    expect(windowPage.has_before).toBe(false)
    expect(windowPage.has_after).toBe(false)
  })

  it('trims on a complete turn boundary when visible turns exceed historyWindowLimit', () => {
    const all: Message[] = []
    let ts = 100
    for (let turn = 0; turn < 100; turn++) {
      all.push(userMessage(`u-${turn}`, ts++, `Prompt ${turn}`))
      all.push(assistantToolCall(`a-${turn}-tool-1`, `c-${turn}-1`, ts++))
      all.push(toolResult(`t-${turn}-tool-1`, `c-${turn}-1`, ts++))
      all.push(assistantToolCall(`a-${turn}-tool-2`, `c-${turn}-2`, ts++))
      all.push(toolResult(`t-${turn}-tool-2`, `c-${turn}-2`, ts++))
      all.push(assistantToolCall(`a-${turn}-tool-3`, `c-${turn}-3`, ts++))
      all.push(toolResult(`t-${turn}-tool-3`, `c-${turn}-3`, ts++))
      all.push(assistantText(`a-${turn}-text`, `Answer ${turn}`, ts++))
    }

    const tail = all.slice(-240)
    const beforeTail = all.slice(0, all.length - 240)
    const current: MessagePage = {
      messages: tail,
      has_before: true,
      has_after: false,
      timing: { completed: {} }
    }
    const merged = mergeMessagePage(
      current,
      {
        messages: beforeTail,
        has_before: false,
        has_after: true,
        timing: { completed: {} }
      },
      'before'
    )

    expect(merged.has_before).toBe(false)
    expect(merged.has_after).toBe(true)
    const lastMessage = merged.messages.at(-1)
    expect(lastMessage?.id).toBe('a-89-text')
    expect(lastMessage?.role).toBe('assistant')
    expect(lastMessage?.parts.some((part) => part.type === 'text')).toBe(true)
  })
})
