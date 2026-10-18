import type { Message } from './contracts'

export function consolidateToolMessages(messages: Message[]): Message[] {
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
  const calls = new Map<string, Message['parts'][number]>()
  for (const row of rows)
    for (const part of row.message.parts) {
      if (part.type === 'tool_call' && part.data.id) calls.set(`${row.turn}:${part.data.id}`, part)
    }
  return rows
    .map((row) => ({
      ...row.message,
      parts: row.message.parts.filter((part) => {
        if (part.type !== 'tool_result' || !part.data.tool_call_id) return true
        const call = calls.get(`${row.turn}:${part.data.tool_call_id}`)
        if (!call) return true
        call.data.result = part.data
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
}
