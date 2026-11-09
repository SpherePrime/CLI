import type { Message } from '../../../../shared/contracts'

function isShellCommand(message: Message) {
  return message.parts.some((part) => part.type === 'shell_command')
}

// A generated continuation (a background job wake-up, a worker notice) arrives
// as a hidden user message. It continues the request the user started, so it
// must not restart the working timer.
function isGeneratedContinuation(message: Message) {
  const texts = message.parts.filter((part) => part.type === 'text')
  return texts.length > 0 && texts.every((part) => Boolean(part.data?.hidden))
}

export function isConversationPrompt(message: Message) {
  return message.role === 'user' && !isShellCommand(message) && !isGeneratedContinuation(message)
}

export function conversationTiming(messages: Message[]) {
  let startedAt: number | null = null
  const completed = new Map<string, number>()
  for (const message of messages) {
    if (!['user', 'assistant'].includes(message.role)) continue
    if (message.role === 'user') {
      if (!isConversationPrompt(message)) continue
      if (message.created_at && Number.isFinite(message.created_at))
        startedAt = message.created_at * 1000
      continue
    }
    if (startedAt === null && message.created_at && Number.isFinite(message.created_at))
      startedAt = message.created_at * 1000
    const finish = message.parts.find((part) => part.type === 'finish')?.data
    if (!finish || finish.reason === 'tool_use') continue
    if (startedAt !== null && Number.isFinite(finish.time) && finish.time > 0)
      completed.set(message.id, Math.max(0, Math.floor(finish.time - startedAt / 1000)))
  }
  return { startedAt, completed }
}

export function isEndOfTurn(messages: Message[], index: number, busy: boolean): boolean {
  const current = messages[index]
  if (!current || current.role !== 'assistant') return false

  for (let i = index + 1; i < messages.length; i++) {
    const next = messages[i]
    if (next.role === 'user' && isConversationPrompt(next)) {
      return true
    }
    if (next.role === 'assistant') {
      return false
    }
  }

  return !busy
}

export function turnCompletedDuration(
  messages: Message[],
  index: number,
  completedMap: Map<string, number>
): number | undefined {
  const direct = completedMap.get(messages[index]?.id)
  if (direct !== undefined) return direct

  for (let i = index - 1; i >= 0; i--) {
    const prev = messages[i]
    if (prev.role === 'user' && isConversationPrompt(prev)) break
    const prevTime = completedMap.get(prev.id)
    if (prevTime !== undefined) return prevTime
  }
  return undefined
}
