import type { Message } from '../../../../shared/contracts'
import type { MergedMessage } from './toolGroupMerge'

function isShellCommand(message: Message) {
  return message.parts.some((part) => part.type === 'shell_command')
}

function isGeneratedContinuation(message: Message) {
  const texts = message.parts.filter((part) => part.type === 'text')
  return texts.length > 0 && texts.every((part) => Boolean(part.data?.hidden))
}

export function isConversationPrompt(message: Message) {
  return message.role === 'user' && !isShellCommand(message) && !isGeneratedContinuation(message)
}

export function conversationTiming(messages: Message[], initialStartedAtSeconds?: number) {
  let startedAt: number | null =
    initialStartedAtSeconds && Number.isFinite(initialStartedAtSeconds)
      ? initialStartedAtSeconds * 1000
      : null
  const completed = new Map<string, number>()
  for (const message of messages) {
    if (message.is_summary_message) continue
    if (!['user', 'assistant'].includes(message.role)) continue
    if (message.role === 'user') {
      if (!isConversationPrompt(message)) continue
      if (message.created_at && Number.isFinite(message.created_at))
        startedAt = message.created_at * 1000
      continue
    }
    if (startedAt === null && message.created_at && Number.isFinite(message.created_at))
      startedAt = message.created_at * 1000
    const finish = [...(message.parts || [])].reverse().find((part) => part.type === 'finish')?.data
    if (!finish || finish.reason === 'tool_use') continue
    const endTime =
      Number.isFinite(finish.time) && finish.time > 0
        ? finish.time
        : message.updated_at && Number.isFinite(message.updated_at)
          ? message.updated_at
          : 0
    if (startedAt !== null && endTime > 0)
      completed.set(message.id, Math.max(0, Math.floor(endTime - startedAt / 1000)))
  }
  return { startedAt, completed }
}

export function isEndOfTurn(messages: Message[], index: number, busy: boolean): boolean {
  const current = messages[index]
  if (!current || current.role !== 'assistant' || current.is_summary_message) return false

  for (let i = index + 1; i < messages.length; i++) {
    const next = messages[i]
    if (next.is_summary_message) continue
    if (next.role === 'user' && isConversationPrompt(next)) {
      return true
    }
    if (next.role === 'assistant') {
      return false
    }
  }

  return !busy
}

function lookupCompletedDuration(
  message: MergedMessage,
  completedMap: Map<string, number>
): number | undefined {
  const direct = completedMap.get(message.id)
  if (direct !== undefined) return direct
  const mergedIds = message._mergedIds
  if (!mergedIds?.length) return undefined
  for (let i = mergedIds.length - 1; i >= 0; i--) {
    const hit = completedMap.get(mergedIds[i])
    if (hit !== undefined) return hit
  }
  return undefined
}

export function turnCompletedDuration(
  messages: MergedMessage[],
  index: number,
  completedMap: Map<string, number>
): number | undefined {
  const current = messages[index]
  if (!current || current.is_summary_message) return undefined
  const direct = lookupCompletedDuration(current, completedMap)
  if (direct !== undefined) return direct

  for (let i = index - 1; i >= 0; i--) {
    const prev = messages[i]
    if (prev.is_summary_message) continue
    if (prev.role === 'user' && isConversationPrompt(prev)) break
    const prevTime = lookupCompletedDuration(prev, completedMap)
    if (prevTime !== undefined) return prevTime
  }
  return undefined
}
