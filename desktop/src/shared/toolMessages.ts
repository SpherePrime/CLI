import type { Message } from './contracts'
import { isBuiltinSkillReadTool } from './builtinSkillTool'

type MessagePart = Message['parts'][number]
type Part = MessagePart

export function hasAgentJobId(metadata: unknown): boolean {
  let value: unknown = metadata
  if (typeof metadata === 'string') {
    if (!metadata) return false
    try {
      value = JSON.parse(metadata)
    } catch {
      return false
    }
  }
  if (!value || typeof value !== 'object') return false
  return Boolean((value as { job_id?: unknown }).job_id)
}

function isFailedAgentLaunch(part: MessagePart, result: any): boolean {
  return (
    part.type === 'tool_call' &&
    part.data?.name === 'agent' &&
    Boolean(result?.is_error) &&
    !hasAgentJobId(result?.metadata)
  )
}

export const hiddenToolNames = new Set(['search_instructions', 'read_instruction', 'todos'])

function hiddenPart(part: Part) {
  if (part.type !== 'tool_call' && part.type !== 'tool_result') return false
  const name = String(part.data?.name || '')
  if (hiddenToolNames.has(name)) return true
  if (part.type === 'tool_call' && isBuiltinSkillReadTool(name, part.data?.input)) return true
  return false
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
    let droppedHiddenTool = false
    const parts: Part[] = []
    for (const part of message.parts || []) {
      if (consumed.has(part)) {
        changed = true
        continue
      }
      if (hiddenPart(part)) {
        changed = true
        droppedHiddenTool = true
        continue
      }
      if (results.has(part)) {
        const result = results.get(part)
        if (isFailedAgentLaunch(part, result)) {
          changed = true
          continue
        }
        parts.push({ ...part, data: { ...part.data, result } })
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
    if (message.role === 'tool' && parts.every((part) => part.type === 'tool_result')) {
      continue
    }
    const onlyNonErrorFinish =
      parts.length > 0 &&
      parts.every(
        (part) => part.type === 'finish' && !['error', 'canceled'].includes(part.data?.reason)
      )
    const keepEmpty =
      Boolean(message.is_summary_message) ||
      (message.role === 'assistant' &&
        parts.some((part) => part.type === 'finish') &&
        !(droppedHiddenTool && onlyNonErrorFinish))
    if (!keepEmpty && !parts.some(visiblePart)) continue
    displayed.push(changed ? { ...message, parts } : message)
  }
  return displayed
}
