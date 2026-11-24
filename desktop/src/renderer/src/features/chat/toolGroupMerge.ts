import type { Message } from '../../../../shared/contracts'

type Part = Message['parts'][number]

function isGroupableToolPart(part: Part): boolean {
  return (
    ['tool_call', 'tool_result', 'shell_command'].includes(part.type) &&
    !(part.type === 'tool_call' && part.data?.name === 'agent')
  )
}

function isMergeableToolOnlyAssistant(message: Message): boolean {
  if (message.role !== 'assistant' || message.is_summary_message) return false
  const parts = message.parts || []
  let hasTool = false
  for (const part of parts) {
    if (isGroupableToolPart(part)) {
      hasTool = true
      continue
    }
    if (part.type === 'finish' && !['error', 'canceled'].includes(part.data?.reason)) {
      continue
    }
    if (part.type === 'text' && (part.data?.hidden || !String(part.data?.text || '').trim())) {
      continue
    }
    return false
  }
  return hasTool
}

export type MessageRenderSegment =
  | { kind: 'part'; part: Part; index: number }
  | { kind: 'tool-group'; parts: Part[]; index: number }

export function partitionMessageParts(parts: Part[]): MessageRenderSegment[] {
  const segments: MessageRenderSegment[] = []
  for (let index = 0; index < parts.length; index += 1) {
    const part = parts[index]
    if (!isGroupableToolPart(part)) {
      segments.push({ kind: 'part', part, index })
      continue
    }
    const startIndex = index
    const group: Part[] = [part]
    while (index + 1 < parts.length && isGroupableToolPart(parts[index + 1])) {
      index += 1
      group.push(parts[index])
    }
    if (group.length >= 2) {
      segments.push({ kind: 'tool-group', parts: group, index: startIndex })
    } else {
      segments.push({ kind: 'part', part: group[0], index: startIndex })
    }
  }
  return segments
}

export function mergeConsecutiveToolMessages(messages: Message[]): Message[] {
  const result: Message[] = []
  for (let index = 0; index < messages.length; index += 1) {
    const current = messages[index]
    if (!isMergeableToolOnlyAssistant(current)) {
      result.push(current)
      continue
    }

    const mergedParts: Part[] = [...(current.parts || []).filter((part) => part.type !== 'finish')]
    let lastFinish: Part | null =
      (current.parts || []).find((part) => part.type === 'finish') || null
    let lastUpdatedAt = current.updated_at
    let mergedCount = 1

    while (index + 1 < messages.length && isMergeableToolOnlyAssistant(messages[index + 1])) {
      index += 1
      mergedCount += 1
      const next = messages[index]
      lastUpdatedAt = next.updated_at || lastUpdatedAt
      lastFinish = null
      for (const part of next.parts || []) {
        if (part.type === 'finish') {
          lastFinish = part
        } else {
          mergedParts.push(part)
        }
      }
    }

    if (mergedCount === 1) {
      result.push(current)
      continue
    }
    if (lastFinish) {
      mergedParts.push(lastFinish)
    }
    result.push({
      ...current,
      updated_at: lastUpdatedAt,
      parts: mergedParts
    })
  }
  return result
}
