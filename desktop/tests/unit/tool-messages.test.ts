import { describe, expect, it } from 'vitest'
import { consolidateToolMessages } from '../../src/shared/toolMessages'
import type { Message } from '../../src/shared/contracts'

const message = (id: string, role: string, parts: any[], extra: Record<string, unknown> = {}) =>
  ({ id, role, parts, ...extra }) as unknown as Message

describe('consolidateToolMessages', () => {
  it('drops user messages that only carry hidden continuations', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Build it' } }]),
      message('a1', 'assistant', [{ type: 'text', data: { text: 'Started workers' } }]),
      message('u2', 'user', [{ type: 'text', data: { text: 'Worker finished', hidden: true } }]),
      message('a2', 'assistant', [{ type: 'text', data: { text: 'All done' } }])
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 'a1', 'a2'])
  })

  it('keeps user messages with attachments', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'image_url', data: { url: 'data:image/png;base64,AA' } }])
    ])
    expect(shown).toHaveLength(1)
  })

  it('preserves in-progress and finished summary messages', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Hello' } }]),
      message('s1', 'assistant', [], { is_summary_message: true }),
      message('s2', 'assistant', [{ type: 'finish', data: { reason: 'end_turn' } }], {
        is_summary_message: true
      }),
      message('s3', 'assistant', [{ type: 'finish', data: { reason: 'error', message: 'boom' } }], {
        is_summary_message: true
      })
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 's1', 's2', 's3'])
  })

  it('preserves finished empty assistant messages while dropping unfinished empty ones', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Hi' } }]),
      message('a-unfinished', 'assistant', [{ type: 'text', data: { text: '' } }]),
      message('a-empty-finished', 'assistant', [{ type: 'finish', data: { reason: 'end_turn' } }])
    ])
    expect(shown.map((item) => item.id)).toEqual(['u1', 'a-empty-finished'])
  })
})
