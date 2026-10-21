import type { Message } from './contracts'

type Part = Message['parts'][number]

export const hiddenToolNames = new Set(['search_instructions', 'read_instruction', 'todos'])

function hiddenPart(part: Part) {
  return (
    (part.type === 'tool_call' || part.type === 'tool_result') &&
    hiddenToolNames.has(String(part.data?.name || ''))
  )
}

function visiblePart(part: Part) {
  if (part.type === 'finish') return ['error', 'canceled'].includes(part.data?.reason)
  if (part.type === 'text') return Boolean(part.data?.text) && !part.data?.hidden
  if (part.type === 'reasoning') return Boolean(part.data?.thinking)
  return true
}

export function consolidateToolMessages(messages: Message[]): Message[] {
  const rows: { turn: string; message: Message }[] = []
  let turn = ''
  for (const message of messages) {
    if (message.role === 'system') continue
    if (message.role === 'user') turn = message.id
    rows.push({ turn, message })
  }
  const calls = new Map<string, Part>()
  for (const { turn, message } of rows)
    for (const part of message.parts || [])
      if (part.type === 'tool_call' && part.data?.id) calls.set(`${turn}:${part.data.id}`, part)
  const results = new Map<Part, unknown>()
  const consumed = new Set<Part>()
  for (const { turn, message } of rows)
    for (const part of message.parts || []) {
      if (part.type !== 'tool_result' || !part.data?.tool_call_id) continue
      const call = calls.get(`${turn}:${part.data.tool_call_id}`)
      if (!call) continue
      results.set(call, part.data)
      consumed.add(part)
    }
  const displayed: Message[] = []
  for (const { message } of rows) {
    let changed = !message.parts
    const parts: Part[] = []
    for (const part of message.parts || []) {
      if (consumed.has(part) || hiddenPart(part)) {
        changed = true
        continue
      }
      if (results.has(part)) {
        parts.push({ ...part, data: { ...part.data, result: results.get(part) } })
        changed = true
        continue
      }
      if (!part.data) {
        parts.push({ ...part, data: {} })
        changed = true
        continue
      }
      parts.push(part)
    }
    if (message.role !== 'user' && !parts.some(visiblePart)) continue
    displayed.push(changed ? { ...message, parts } : message)
  }
  return displayed
}
