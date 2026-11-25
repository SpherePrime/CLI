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

  it('drops orphaned tool-only messages across pagination boundaries while keeping matched tool results', () => {
    const shown = consolidateToolMessages([
      message('orphan-tool', 'tool', [
        { type: 'tool_result', data: { tool_call_id: 'missing-call', name: 'view', content: 'old file' } }
      ]),
      message('a1', 'assistant', [
        { type: 'tool_call', data: { id: 'c1', name: 'view', input: '{"file_path":"a.ts"}' } }
      ]),
      message('t1', 'tool', [
        { type: 'tool_result', data: { tool_call_id: 'c1', name: 'view', content: 'ok' } }
      ])
    ])
    expect(shown.map((item) => item.id)).toEqual(['a1'])
  })

  it('hides builtin Prime skill reads in chat while keeping user skills and project files visible', () => {
    const shown = consolidateToolMessages([
      message('u1', 'user', [{ type: 'text', data: { text: 'Make a game UI' } }]),
      message('a-builtin-only', 'assistant', [
        {
          type: 'tool_call',
          data: {
            id: 'call-1',
            name: 'view',
            input: JSON.stringify({ file_path: 'prime://skills/prime-designer/SKILL.md' })
          }
        },
        {
          type: 'tool_call',
          data: {
            id: 'call-2',
            name: 'view',
            input: '{"file_path":"C:\\\\Users\\\\me\\\\AppData\\\\Local\\\\prime\\\\builtin-skills\\\\prime-gamedev\\\\SKILL.md"}'
          }
        },
        {
          type: 'tool_call',
          data: {
            id: 'call-streaming',
            name: 'view',
            input: '{"file_path":"prime://skills/prime-config/SKILL'
          }
        },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      message('t-builtin-only', 'tool', [
        { type: 'tool_result', data: { tool_call_id: 'call-1', name: 'view', content: 'designer docs' } },
        { type: 'tool_result', data: { tool_call_id: 'call-2', name: 'view', content: 'gamedev docs' } }
      ]),
      message('a-mixed', 'assistant', [
        {
          type: 'tool_call',
          data: {
            id: 'call-3',
            name: 'view',
            input: { file_path: 'prime://skills/prime-designer/references/quality-gate.md' }
          }
        },
        {
          type: 'tool_call',
          data: {
            id: 'call-4',
            name: 'view',
            input: JSON.stringify({
              file_path: 'C:\\Users\\me\\.config\\prime\\skills\\superpowers\\brainstorming\\SKILL.md'
            })
          }
        },
        {
          type: 'tool_call',
          data: {
            id: 'call-5',
            name: 'view',
            input: JSON.stringify({ file_path: 'src/App.tsx' })
          }
        },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]),
      message('t-mixed', 'tool', [
        { type: 'tool_result', data: { tool_call_id: 'call-3', name: 'view', content: 'builtin ref' } },
        { type: 'tool_result', data: { tool_call_id: 'call-4', name: 'view', content: 'user skill' } },
        { type: 'tool_result', data: { tool_call_id: 'call-5', name: 'view', content: 'app code' } }
      ])
    ])

    expect(shown.map((item) => item.id)).toEqual(['u1', 'a-mixed'])
    const mixedCalls = shown[1].parts.filter((part) => part.type === 'tool_call')
    expect(mixedCalls.map((part) => part.data.id)).toEqual(['call-4', 'call-5'])
  })
})
