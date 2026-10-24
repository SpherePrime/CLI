import { describe, expect, it } from 'vitest'
import { consolidateToolMessages } from '../../src/shared/toolMessages'
import type { Message } from '../../src/shared/contracts'

const message = (id: string, role: string, parts: any[]) => ({ id, role, parts }) as unknown as Message

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
})
