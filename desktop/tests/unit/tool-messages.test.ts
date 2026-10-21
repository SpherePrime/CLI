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

it('returns untouched messages by reference', () => {
  const user = msg('u', 'user', [{ type: 'text', data: { text: 'go' } }])
  const text = msg('t', 'assistant', [{ type: 'text', data: { text: 'answer' } }])
  const withCall = msg('a', 'assistant', [call('one')])
  const displayed = consolidateToolMessages([
    user,
    text,
    withCall,
    msg('r', 'tool', [result('one', 'done')])
  ])
  expect(displayed[0]).toBe(user)
  expect(displayed[1]).toBe(text)
  expect(displayed[2]).not.toBe(withCall)
  expect(displayed[2].parts[0].data.result.content).toBe('done')
})

function previousConsolidate(messages: Message[]): Message[] {
  const hidden = new Set(['search_instructions', 'read_instruction', 'todos'])
  const hiddenPart = (part: any) =>
    (part.type === 'tool_call' || part.type === 'tool_result') &&
    hidden.has(String(part.data?.name || ''))
  const visiblePart = (part: any) => {
    if (part.type === 'finish') return ['error', 'canceled'].includes(part.data?.reason)
    if (part.type === 'text') return Boolean(part.data?.text) && !part.data?.hidden
    if (part.type === 'reasoning') return Boolean(part.data?.thinking)
    return true
  }
  let turn = ''
  const rows = messages
    .filter((message) => message.role !== 'system')
    .map((message) => {
      if (message.role === 'user') turn = message.id
      return {
        turn,
        message: {
          ...message,
          parts: (message.parts || []).map((part) => ({ ...part, data: { ...part.data } }))
        }
      }
    })
  const calls = new Map<string, any>()
  for (const row of rows)
    for (const part of row.message.parts)
      if (part.type === 'tool_call' && part.data.id) calls.set(`${row.turn}:${part.data.id}`, part)
  return rows
    .map((row) => ({
      ...row.message,
      parts: row.message.parts.filter((part) => {
        if (part.type !== 'tool_result' || !part.data.tool_call_id) return true
        const found = calls.get(`${row.turn}:${part.data.tool_call_id}`)
        if (!found) return true
        found.data.result = part.data
        return false
      })
    }))
    .filter(
      (message) =>
        message.role !== 'tool' ||
        message.parts.some(
          (part) => part.type !== 'finish' || ['error', 'canceled'].includes(part.data.reason)
        )
    )
    .map((message) => ({ ...message, parts: message.parts.filter((part) => !hiddenPart(part)) }))
    .filter((message) => message.role === 'user' || message.parts.some(visiblePart))
}

it('matches the previous consolidation on mixed histories', () => {
  let seed = 7
  const random = (limit: number) => {
    seed = (seed * 1103515245 + 12345) % 2147483648
    return seed % limit
  }
  const names = ['bash', 'ls', 'todos', 'read_instruction']
  const ids = ['c0', 'c1', 'c2']
  for (let round = 0; round < 300; round++) {
    const source: Message[] = []
    const count = 2 + random(10)
    for (let index = 0; index < count; index++) {
      const role = ['user', 'assistant', 'tool', 'system'][random(4)]
      const parts: Message['parts'] = []
      const partCount = random(4)
      for (let item = 0; item < partCount; item++) {
        const kind = random(6)
        const name = names[random(names.length)]
        const id = ids[random(ids.length)]
        if (kind === 0) parts.push({ type: 'tool_call', data: { id, name, input: '{}' } })
        if (kind === 1)
          parts.push({ type: 'tool_result', data: { tool_call_id: id, name, content: `r${round}` } })
        if (kind === 2) parts.push({ type: 'text', data: { text: random(2) ? 'x' : '' } })
        if (kind === 3)
          parts.push({ type: 'finish', data: { reason: ['stop', 'error', 'tool_use'][random(3)] } })
        if (kind === 4) parts.push({ type: 'reasoning', data: { thinking: random(2) ? 't' : '' } })
        if (kind === 5) parts.push({ type: 'binary', data: { path: 'p' } })
      }
      source.push(msg(`m${index}`, role, parts))
    }
    expect(consolidateToolMessages(source)).toEqual(previousConsolidate(source))
  }
})
