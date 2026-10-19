import { describe, expect, it } from 'vitest'
import {
  exportConversation,
  findMessages,
  groupSessions,
  safeExportName
} from '../../src/shared/workspaceTools'
import type { Message, Session } from '../../src/shared/contracts'

const messages: Message[] = [
  {
    id: 'one',
    session_id: 's',
    role: 'user',
    parts: [{ type: 'text', data: { text: 'Найди ошибку' } }]
  },
  {
    id: 'two',
    session_id: 's',
    role: 'assistant',
    parts: [
      { type: 'text', data: { text: 'Ошибка в обработчике.' } },
      { type: 'text', data: { text: 'private token', hidden: true } },
      { type: 'reasoning', data: { thinking: 'private reasoning' } }
    ]
  },
  {
    id: 'tool',
    session_id: 's',
    role: 'tool',
    parts: [{ type: 'tool_result', data: { content: 'private output' } }]
  }
]

describe('conversation tools', () => {
  it('searches visible text case-insensitively and excludes hidden content and tools', () => {
    expect(findMessages(messages, ' ОШИБК ')).toEqual(['one', 'two'])
    expect(findMessages(messages, 'private')).toEqual([])
    expect(findMessages(messages, '  ')).toEqual([])
  })

  it('exports readable messages without hidden text, reasoning or tool payloads', () => {
    expect(exportConversation('Debug', messages, { user: 'You', assistant: 'Prime' })).toBe(
      '# Debug\n\n## You\n\nНайди ошибку\n\n## Prime\n\nОшибка в обработчике.\n'
    )
  })

  it('creates a bounded portable filename from a chat title', () => {
    expect(safeExportName('  fix: <app>/test?  ')).toBe('fix- -app--test-.md')
    expect(safeExportName('...')).toBe('chat.md')
    expect(safeExportName('x'.repeat(200)).length).toBeLessThanOrEqual(83)
  })
})

describe('chat groups', () => {
  it('keeps pins separate and groups by local calendar days without mutating sessions', () => {
    const now = new Date(2026, 9, 19, 0, 30)
    const seconds = (day: number) => new Date(2026, 9, day, 23).getTime() / 1000
    const sessions: Session[] = [
      { id: 'old', title: 'Old', updated_at: seconds(1) },
      { id: 'yesterday', title: 'Yesterday', updated_at: seconds(18) },
      { id: 'pin', title: 'Pin', updated_at: seconds(1) },
      { id: 'today', title: 'Today', updated_at: now.getTime() / 1000 },
      { id: 'child', title: 'Child', parent_session_id: 'today' }
    ]
    expect(
      groupSessions(sessions, ['pin'], '', now).map((group) => [
        group.id,
        group.sessions.map((s) => s.id)
      ])
    ).toEqual([
      ['pinned', ['pin']],
      ['today', ['today']],
      ['yesterday', ['yesterday']],
      ['older', ['old']]
    ])
    expect(sessions[0].id).toBe('old')
    expect(
      groupSessions(sessions, ['pin'], 'yEsTeR', now).flatMap((group) =>
        group.sessions.map((s) => s.id)
      )
    ).toEqual(['yesterday'])
  })
})
