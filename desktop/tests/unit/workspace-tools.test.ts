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
    created_at: new Date(2026, 9, 20, 14, 2).getTime() / 1000,
    parts: [{ type: 'text', data: { text: 'Найди ошибку' } }]
  },
  {
    id: 'two',
    session_id: 's',
    role: 'assistant',
    created_at: new Date(2026, 9, 20, 14, 4).getTime() / 1000,
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

  it('exports a styled document without hidden text, reasoning or tool payloads', () => {
    const now = new Date(2026, 9, 20, 14, 30)
    expect(
      exportConversation(
        'Debug',
        messages,
        { user: 'User', assistant: 'Prime', exported: 'Export: {value0}' },
        now
      )
    ).toBe(
      '# Debug\n\n*Export: 20.10.2026, 14:30*\n\n---\n\n**User** · 14:02\n\n> Найди ошибку\n\n---\n\n**Prime** · 14:04\n\nОшибка в обработчике.\n'
    )
  })

  it('keeps multiline user quotes and omits timestamps without created_at', () => {
    const plain: Message[] = [
      {
        id: 'q',
        session_id: 's',
        role: 'user',
        parts: [{ type: 'text', data: { text: 'Строка одна\n\nСтрока две' } }]
      }
    ]
    expect(exportConversation('T', plain, { user: 'User', assistant: 'Prime' })).toBe(
      '# T\n\n---\n\n**User**\n\n> Строка одна\n>\n> Строка две\n'
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
