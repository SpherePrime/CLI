import { expect, it } from 'vitest'
import { consolidateToolMessages } from '../../src/shared/toolMessages'
import type { Message } from '../../src/shared/contracts'

const msg = (id: string, role: string, parts: Message['parts']): Message => ({
  id,
  role,
  parts,
  session_id: 's'
})
const call = (id: string) => ({
  type: 'tool_call',
  data: { id, name: 'bash', input: '{"command":"pwd"}', finished: true }
})
const result = (id: string, content: string) => ({
  type: 'tool_result',
  data: { tool_call_id: id, name: 'bash', content, is_error: false }
})

it('hides a consumed tool message that only retains its normal finish marker', () => {
  const displayed = consolidateToolMessages([
    msg('a', 'assistant', [call('one')]),
    msg('b', 'tool', [result('one', 'done'), { type: 'finish', data: { reason: 'stop' } }])
  ])
  expect(displayed.map((message) => message.id)).toEqual(['a'])
})

it('joins commands and output by call ID without altering stored messages', () => {
  const source = [
    msg('a', 'assistant', [call('one'), call('two')]),
    msg('b', 'tool', [result('two', 'second')]),
    msg('c', 'tool', [result('one', 'first')])
  ]
  const before = JSON.stringify(source)
  const displayed = consolidateToolMessages(source)
  expect(displayed).toHaveLength(1)
  expect(displayed[0].parts[0].data.result.content).toBe('first')
  expect(displayed[0].parts[1].data.result.content).toBe('second')
  expect(JSON.stringify(source)).toBe(before)
})

it('keeps unmatched output and other parts of tool messages', () => {
  const source = [
    msg('a', 'assistant', [call('one')]),
    msg('b', 'tool', [
      result('one', 'done'),
      result('missing', 'orphan'),
      { type: 'text', data: { text: 'extra' } }
    ])
  ]
  const displayed = consolidateToolMessages(source)
  expect(displayed[1].parts).toEqual(source[1].parts.slice(1))
})

it('isolates reused tool IDs between user turns and reconciles early result events', () => {
  const displayed = consolidateToolMessages([
    msg('u1', 'user', []),
    msg('r1', 'tool', [result('same', 'old')]),
    msg('a1', 'assistant', [call('same')]),
    msg('u2', 'user', []),
    msg('a2', 'assistant', [call('same')]),
    msg('r2', 'tool', [result('same', 'new')])
  ])
  expect(displayed.find((m) => m.id === 'a1')!.parts[0].data.result.content).toBe('old')
  expect(displayed.find((m) => m.id === 'a2')!.parts[0].data.result.content).toBe('new')
})

it('hides internal instruction and todo tools and steps left empty by them', () => {
  const displayed = consolidateToolMessages([
    { id: 'u', session_id: 's', role: 'user', parts: [{ type: 'text', data: { text: 'go' } }] },
    {
      id: 'a1',
      session_id: 's',
      role: 'assistant',
      parts: [
        { type: 'tool_call', data: { id: 'c1', name: 'read_instruction', input: '{}' } },
        { type: 'finish', data: { reason: 'tool_use' } }
      ]
    },
    {
      id: 't1',
      session_id: 's',
      role: 'tool',
      parts: [{ type: 'tool_result', data: { tool_call_id: 'c1', name: 'read_instruction' } }]
    },
    {
      id: 'a2',
      session_id: 's',
      role: 'assistant',
      parts: [
        { type: 'tool_call', data: { id: 'c2', name: 'todos', input: '{}' } },
        { type: 'tool_call', data: { id: 'c3', name: 'ls', input: '{}' } }
      ]
    }
  ])
  expect(displayed.map((message) => message.id)).toEqual(['u', 'a2'])
  expect(displayed[1].parts.map((part) => part.data.name)).toEqual(['ls'])
})
