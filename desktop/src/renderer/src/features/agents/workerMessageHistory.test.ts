import { describe, expect, it, vi } from 'vitest'
import type { Message } from '../../../../shared/contracts'
import { operationRoute } from '../../../../shared/operations'
import { historyWindowLimit, type MessagePage } from '../chat/messageWindow'
import {
  createWorkerMessageHistory,
  type WorkerMessageHistoryState
} from './workerMessageHistory'

function makeMessage(index: number): Message {
  return {
    id: `msg-${index}`,
    session_id: 'worker-sess-1',
    role: index % 2 === 0 ? 'user' : 'assistant',
    created_at: 1000 + index,
    parts: [{ type: 'text', data: { text: `Message ${index}` } }]
  }
}

function makeMessages(start: number, count: number): Message[] {
  return Array.from({ length: count }, (_, offset) => makeMessage(start + offset))
}

describe('worker message route and pagination', () => {
  it('builds paginated agentWorkerMessages routes', () => {
    expect(
      operationRoute('agentWorkerMessages', {
        sessionId: 'sess-1',
        workerId: 'w-1'
      })
    ).toEqual(['GET', 'agent/sessions/sess-1/workers/w-1/messages?limit=60'])

    expect(
      operationRoute('agentWorkerMessages', {
        sessionId: 'sess-1',
        workerId: 'w-1',
        limit: 40,
        before: 'msg-60',
        after: 'msg-1',
        around: 'msg-30',
        textOnly: true
      })
    ).toEqual([
      'GET',
      'agent/sessions/sess-1/workers/w-1/messages?limit=40&before=msg-60&after=msg-1&around=msg-30&text_only=true'
    ])
  })

  it('loads initial page, paginates before, and skips poll while scrolled up with hasAfter', async () => {
    const all = makeMessages(0, 240)
    const slicePage = (args: { limit: number; before?: string; after?: string }): MessagePage => {
      if (args.before) {
        const end = all.findIndex((item) => item.id === args.before)
        const start = Math.max(0, end - args.limit)
        return {
          messages: all.slice(start, end),
          has_before: start > 0,
          has_after: end < all.length,
          timing: { completed: {} }
        }
      }
      if (args.after) {
        const idx = all.findIndex((item) => item.id === args.after)
        const start = idx + 1
        const end = Math.min(all.length, start + args.limit)
        return {
          messages: all.slice(start, end),
          has_before: start > 0,
          has_after: end < all.length,
          timing: { completed: {} }
        }
      }
      const start = Math.max(0, all.length - args.limit)
      return {
        messages: all.slice(start),
        has_before: start > 0,
        has_after: false,
        timing: { completed: {} }
      }
    }

    const fetchPage = vi.fn(async (args: { limit: number; before?: string; after?: string }) =>
      slicePage(args)
    )
    let latestState!: WorkerMessageHistoryState
    const history = createWorkerMessageHistory({
      fetchPage,
      onChange: (next) => {
        latestState = next
      }
    })

    await history.poll('running:1', true)
    expect(fetchPage).toHaveBeenCalledWith({ limit: 60 })
    expect(latestState.initialLoading).toBe(false)
    expect(latestState.messages).toHaveLength(60)
    expect(latestState.messages[0].id).toBe('msg-180')
    expect(latestState.hasBefore).toBe(true)
    expect(latestState.hasAfter).toBe(false)

    await history.load('before')
    expect(fetchPage).toHaveBeenLastCalledWith({ limit: 60, before: 'msg-180' })
    expect(latestState.messages).toHaveLength(120)
    expect(latestState.messages[0].id).toBe('msg-120')
    expect(latestState.hasAfter).toBe(false)

    await history.load('before')
    expect(latestState.messages).toHaveLength(180)
    expect(latestState.messages[0].id).toBe('msg-60')
    expect(latestState.hasAfter).toBe(false)

    await history.load('before')
    expect(latestState.messages).toHaveLength(historyWindowLimit)
    expect(latestState.messages[0].id).toBe('msg-0')
    expect(latestState.messages.at(-1)?.id).toBe('msg-179')
    expect(latestState.hasBefore).toBe(false)
    expect(latestState.hasAfter).toBe(true)

    const callsBeforePoll = fetchPage.mock.calls.length
    await history.poll('running:2', true)
    expect(fetchPage).toHaveBeenCalledTimes(callsBeforePoll)
    expect(latestState.messages[0].id).toBe('msg-0')

    await history.load('after')
    expect(fetchPage).toHaveBeenLastCalledWith({ limit: 60, after: 'msg-179' })
    expect(latestState.messages.at(-1)?.id).toBe('msg-239')
    expect(latestState.hasAfter).toBe(false)

    const tailRef = latestState.messages.at(-1)
    all.push(makeMessage(240))
    await history.poll('running:3', true)
    expect(latestState.messages.at(-1)?.id).toBe('msg-240')
    expect(latestState.messages.at(-2)).toBe(tailRef)
  })

  it('skips redundant polling for inactive workers until revision changes or resets', async () => {
    const fetchPage = vi.fn(async () => ({
      messages: makeMessages(0, 3),
      has_before: false,
      has_after: false,
      timing: { completed: {} }
    }))
    const onChange = vi.fn()
    const history = createWorkerMessageHistory({ fetchPage, onChange })

    await history.poll('done:10', false)
    expect(fetchPage).toHaveBeenCalledTimes(1)

    await history.poll('done:10', false)
    expect(fetchPage).toHaveBeenCalledTimes(1)

    history.resetRevision()
    await history.poll('done:10', false)
    expect(fetchPage).toHaveBeenCalledTimes(2)

    await history.poll('done:11', false)
    expect(fetchPage).toHaveBeenCalledTimes(3)
  })

  it('surfaces initial load errors and clears them on retry', async () => {
    const fetchPage = vi
      .fn()
      .mockRejectedValueOnce(new Error('network down'))
      .mockResolvedValueOnce({
        messages: makeMessages(0, 2),
        has_before: false,
        has_after: false,
        timing: { completed: {} }
      })
    let state!: WorkerMessageHistoryState
    const history = createWorkerMessageHistory({
      fetchPage,
      onChange: (next) => {
        state = next
      }
    })

    await history.poll('running:1', true)
    expect(state.initialLoading).toBe(false)
    expect(state.error).toBe('network down')
    expect(state.messages).toHaveLength(0)

    await history.poll('running:1', true)
    expect(state.error).toBe('')
    expect(state.messages).toHaveLength(2)
  })
})
