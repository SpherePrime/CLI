import type { Message } from '../../../../shared/contracts'

export function conversationTiming(messages: Message[]) {
  let startedAt: number | null = null
  const completed = new Map<string, number>()
  for (const message of messages) {
    if (!['user', 'assistant'].includes(message.role)) continue
    if (message.role === 'user' && message.parts.some((part) => part.type === 'shell_command'))
      continue
    if (startedAt === null && message.created_at && Number.isFinite(message.created_at))
      startedAt = message.created_at * 1000
    if (message.role !== 'assistant') continue
    const finish = message.parts.find((part) => part.type === 'finish')?.data
    if (!finish || finish.reason === 'tool_use') continue
    if (startedAt !== null && Number.isFinite(finish.time) && finish.time > 0)
      completed.set(message.id, Math.max(0, Math.floor(finish.time - startedAt / 1000)))
    startedAt = null
  }
  return { startedAt, completed }
}
