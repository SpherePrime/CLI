import type { Message } from '../../../../shared/contracts'

type Part = Message['parts'][number]

export type MergedMessage = Message & {
  _mergedIds?: string[]
}

function isGroupableToolPart(part: Part): boolean {
  return ['tool_call', 'tool_result', 'shell_command'].includes(part.type)
}

function isMergeableAssistantTurn(message: Message): boolean {
  return message.role === 'assistant' && !message.is_summary_message
}

function isVisibleTextPart(part: Part): boolean {
  return part.type === 'text' && !part.data?.hidden && Boolean(part.data?.text?.trim())
}

function isMediaPart(part: Part): boolean {
  return part.type === 'image_url' || part.type === 'binary'
}

function isErrorOrCanceledFinish(part: Part): boolean {
  return (
    part.type === 'finish' &&
    (part.data?.reason === 'error' || part.data?.reason === 'canceled')
  )
}

function normalizeTurnParts(parts: Part[]): Part[] {
  let latestReasoningIndex = -1
  let fallbackReasoningIndex = -1
  let latestErrorFinish: Part | null = null
  let lastFinish: Part | null = null

  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]
    if (part.type === 'reasoning') {
      fallbackReasoningIndex = i
      if (part.data?.thinking?.trim() || !part.data?.finished_at) {
        latestReasoningIndex = i
      }
      continue
    }
    if (part.type === 'finish') {
      if (isErrorOrCanceledFinish(part)) {
        latestErrorFinish = part
      }
      lastFinish = part
    }
  }

  const chosenReasoningIndex =
    latestReasoningIndex !== -1 ? latestReasoningIndex : fallbackReasoningIndex

  const normalized: Part[] = []
  let pendingToolsStart = -1

  for (let i = 0; i < parts.length; i++) {
    const part = parts[i]
    if (isVisibleTextPart(part) || isMediaPart(part)) {
      pendingToolsStart = -1
      normalized.push(part)
      continue
    }
    if (isGroupableToolPart(part)) {
      if (pendingToolsStart === -1) {
        pendingToolsStart = normalized.length
      }
      normalized.push(part)
      continue
    }
    if (part.type === 'reasoning' && i === chosenReasoningIndex) {
      let hasFollowingToolBeforeContent = false
      if (pendingToolsStart !== -1) {
        for (let j = i + 1; j < parts.length; j++) {
          const next = parts[j]
          if (isVisibleTextPart(next) || isMediaPart(next)) break
          if (isGroupableToolPart(next)) {
            hasFollowingToolBeforeContent = true
            break
          }
        }
      }
      if (hasFollowingToolBeforeContent) {
        normalized.splice(pendingToolsStart, 0, part)
        pendingToolsStart += 1
      } else {
        normalized.push(part)
      }
    }
  }

  if (latestErrorFinish) {
    normalized.push(latestErrorFinish)
  } else if (lastFinish) {
    normalized.push(lastFinish)
  }

  return normalized
}

export function mergeConsecutiveToolMessages(messages: Message[]): MergedMessage[] {
  const merged: MergedMessage[] = []
  let index = 0

  while (index < messages.length) {
    const current = messages[index]
    if (!isMergeableAssistantTurn(current)) {
      merged.push(current)
      index += 1
      continue
    }

    const turnMessages: Message[] = [current]
    let nextIndex = index + 1
    while (nextIndex < messages.length && isMergeableAssistantTurn(messages[nextIndex])) {
      turnMessages.push(messages[nextIndex])
      nextIndex += 1
    }

    const combinedParts: Part[] = []
    const mergedIds: string[] = []
    let orchestratorPhase = current.orchestrator_phase

    for (let t = 0; t < turnMessages.length; t++) {
      const turnMessage = turnMessages[t]
      const isLastInTurn = t === turnMessages.length - 1
      mergedIds.push(turnMessage.id)
      if (turnMessage.orchestrator_phase !== undefined) {
        orchestratorPhase = turnMessage.orchestrator_phase
      }

      for (const part of turnMessage.parts || []) {
        if (part.type === 'finish' && !isLastInTurn && !isErrorOrCanceledFinish(part)) {
          continue
        }
        if (part.type === 'tool_call' && part.data?.name === 'agent' && turnMessage.id !== current.id) {
          combinedParts.push({
            ...part,
            data: {
              ...part.data,
              _sourceMessageId: turnMessage.id
            }
          } as Part)
        } else {
          combinedParts.push(part)
        }
      }
    }

    const normalizedParts = normalizeTurnParts(combinedParts)

    merged.push({
      ...current,
      parts: normalizedParts,
      orchestrator_phase: orchestratorPhase,
      _mergedIds: mergedIds.length > 1 ? mergedIds : undefined
    })

    index = nextIndex
  }

  return merged
}

export type MessageSegment =
  | { kind: 'part'; part: Part; index: number }
  | { kind: 'tool-group'; parts: Part[]; index: number }

export function partitionMessageParts(parts: Part[]): MessageSegment[] {
  const normalized = normalizeTurnParts(parts)
  const segments: MessageSegment[] = []
  const toolParts: Part[] = []
  let firstToolIndex = -1

  for (let i = 0; i < normalized.length; i++) {
    const part = normalized[i]
    if (isGroupableToolPart(part)) {
      if (firstToolIndex === -1) firstToolIndex = i
      toolParts.push(part)
    } else {
      if (toolParts.length > 0) {
        segments.push({ kind: 'tool-group', parts: toolParts.splice(0), index: firstToolIndex })
        firstToolIndex = -1
      }
      segments.push({ kind: 'part', part, index: i })
    }
  }

  if (toolParts.length > 0) {
    segments.push({ kind: 'tool-group', parts: toolParts, index: firstToolIndex })
  }

  return segments
}
