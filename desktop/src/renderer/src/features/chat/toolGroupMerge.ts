import type { Message } from '../../../../shared/contracts'

type Part = Message['parts'][number]

function isGroupableToolPart(part: Part): boolean {
  return ['tool_call', 'tool_result', 'shell_command'].includes(part.type)
}

function isNonEmptyReasoning(part: Part): boolean {
  return part.type === 'reasoning' && Boolean(String(part.data?.thinking || '').trim())
}

function hasVisibleTextOrMedia(part: Part): boolean {
  if (part.type === 'text') {
    return !part.data?.hidden && Boolean(String(part.data?.text || '').trim())
  }
  if (part.type === 'image_url' || part.type === 'binary') return true
  if (part.type === 'finish') {
    return ['error', 'canceled'].includes(String(part.data?.reason || ''))
  }
  return false
}

function isMergeableWorkAssistant(message: Message): boolean {
  if (message.role !== 'assistant' || message.is_summary_message) return false
  const parts = message.parts || []
  let hasWork = false
  for (const part of parts) {
    if (isGroupableToolPart(part) || isNonEmptyReasoning(part)) {
      hasWork = true
      continue
    }
    if (part.type === 'reasoning') continue
    if (part.type === 'finish' && !['error', 'canceled'].includes(String(part.data?.reason || ''))) {
      continue
    }
    if (part.type === 'text' && (part.data?.hidden || !String(part.data?.text || '').trim())) {
      continue
    }
    return false
  }
  return hasWork
}

function collapseReasoningBeforeTools(parts: Part[]): Part[] {
  let latestReasoning: Part | null = null
  const nonReasoning: Part[] = []
  for (const part of parts) {
    if (part.type === 'reasoning') {
      if (isNonEmptyReasoning(part)) {
        latestReasoning = part
      }
      continue
    }
    nonReasoning.push(part)
  }
  return latestReasoning ? [latestReasoning, ...nonReasoning] : nonReasoning
}

export type MessageRenderSegment =
  | { kind: 'part'; part: Part; index: number }
  | { kind: 'tool-group'; parts: Part[]; index: number }

export function partitionMessageParts(parts: Part[]): MessageRenderSegment[] {
  const segments: MessageRenderSegment[] = []
  let index = 0

  while (index < parts.length) {
    const current = parts[index]
    if (hasVisibleTextOrMedia(current)) {
      segments.push({ kind: 'part', part: current, index })
      index += 1
      continue
    }

    const startIndex = index
    let latestReasoning: Part | null = null
    let latestReasoningIndex = startIndex
    const tools: Part[] = []
    const trailing: { part: Part; index: number }[] = []

    while (index < parts.length && !hasVisibleTextOrMedia(parts[index])) {
      const part = parts[index]
      if (part.type === 'reasoning') {
        if (isNonEmptyReasoning(part)) {
          latestReasoning = part
          latestReasoningIndex = index
        }
      } else if (isGroupableToolPart(part)) {
        tools.push(part)
      } else {
        trailing.push({ part, index })
      }
      index += 1
    }

    if (latestReasoning) {
      segments.push({ kind: 'part', part: latestReasoning, index: latestReasoningIndex })
    }
    if (tools.length >= 1) {
      segments.push({ kind: 'tool-group', parts: tools, index: startIndex })
    }
    for (const item of trailing) {
      segments.push({ kind: 'part', part: item.part, index: item.index })
    }
  }

  return segments
}

function withSourceMessageId(part: Part, messageId: string): Part {
  if (part.type !== 'tool_call' || part.data?.name !== 'agent') return part
  if (part.data?._sourceMessageId) return part
  return {
    ...part,
    data: {
      ...part.data,
      _sourceMessageId: messageId
    }
  }
}

export function mergeConsecutiveToolMessages(messages: Message[]): Message[] {
  const result: Message[] = []
  for (let index = 0; index < messages.length; index += 1) {
    const current = messages[index]
    if (!isMergeableWorkAssistant(current)) {
      const hasMultipleReasoning =
        (current.parts || []).filter((part) => isNonEmptyReasoning(part)).length > 1
      result.push(
        hasMultipleReasoning
          ? { ...current, parts: collapseReasoningBeforeTools(current.parts || []) }
          : current
      )
      continue
    }

    const rawParts: Part[] = (current.parts || [])
      .filter((part) => part.type !== 'finish')
      .map((part) => withSourceMessageId(part, current.id))
    let lastFinish: Part | null =
      (current.parts || []).find((part) => part.type === 'finish') || null
    let lastUpdatedAt = current.updated_at

    while (index + 1 < messages.length && isMergeableWorkAssistant(messages[index + 1])) {
      index += 1
      const next = messages[index]
      lastUpdatedAt = next.updated_at || lastUpdatedAt
      lastFinish = null
      for (const part of next.parts || []) {
        if (part.type === 'finish') {
          lastFinish = part
        } else {
          rawParts.push(withSourceMessageId(part, next.id))
        }
      }
    }

    const nextMessage = messages[index + 1]
    const nextAssistantHasReasoning =
      nextMessage?.role === 'assistant' &&
      !nextMessage.is_summary_message &&
      (nextMessage.parts || []).some((part) => isNonEmptyReasoning(part))

    const collapsedParts = collapseReasoningBeforeTools(rawParts).filter(
      (part) => !(nextAssistantHasReasoning && part.type === 'reasoning')
    )
    if (lastFinish) {
      collapsedParts.push(lastFinish)
    }

    if (collapsedParts.length === 0) {
      continue
    }

    result.push({
      ...current,
      updated_at: lastUpdatedAt,
      parts: collapsedParts
    })
  }
  return result
}
