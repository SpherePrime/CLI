import type { Message } from '../../../../shared/contracts'
import { applyMessage } from '../../../../shared/messages'

export type HistoryTiming = { started_at?: number; completed: Record<string, number> }
export type MessagePage = {
  messages: Message[]
  has_before: boolean
  has_after: boolean
  timing: HistoryTiming
}
export type PageDirection = 'latest' | 'before' | 'after'
export const historyWindowLimit = 180

export function mergeMessagePage(
  current: MessagePage | undefined,
  page: MessagePage,
  direction: PageDirection
): MessagePage {
  const existing = current?.messages || []
  const incoming = new Map(page.messages.map((message) => [message.id, message]))
  const overlap = existing.some((message) => incoming.has(message.id))
  const historicalRefresh =
    direction === 'latest' &&
    Boolean(current?.has_after || (existing.length && !overlap && page.has_before))
  let messages: Message[]
  if (!current) messages = page.messages
  else if (historicalRefresh && !overlap) messages = existing
  else if (direction === 'before')
    messages = [
      ...page.messages.filter((message) => !existing.some((item) => item.id === message.id)),
      ...existing.map((message) => incoming.get(message.id) || message)
    ]
  else if (direction === 'latest' && current.has_after)
    messages = existing.map((message) => incoming.get(message.id) || message)
  else
    messages = [
      ...existing.map((message) => incoming.get(message.id) || message),
      ...page.messages.filter((message) => !existing.some((item) => item.id === message.id))
    ]
  const trimmed = messages.length > historyWindowLimit
  messages =
    direction === 'before'
      ? messages.slice(0, historyWindowLimit)
      : messages.slice(-historyWindowLimit)
  const completed = { ...current?.timing.completed, ...page.timing?.completed }
  const retained = new Set(messages.map((message) => message.id))
  return {
    messages,
    has_before:
      direction === 'before' || !current ? page.has_before : Boolean(current.has_before || trimmed),
    has_after:
      direction === 'before'
        ? Boolean(current?.has_after || trimmed)
        : historicalRefresh
          ? true
          : page.has_after,
    timing: {
      started_at: page.timing?.started_at,
      completed: Object.fromEntries(Object.entries(completed).filter(([id]) => retained.has(id)))
    }
  }
}

export function updateHistoryMessage(
  window: MessagePage,
  message: Message,
  action: string
): MessagePage {
  const exists = window.messages.some((item) => item.id === message.id)
  if (!exists && (window.has_after || action !== 'created')) return window
  if (
    !exists &&
    message.created_at &&
    window.messages[0]?.created_at &&
    message.created_at < window.messages[0].created_at!
  )
    return window
  const messages =
    action === 'deleted'
      ? window.messages.filter((item) => item.id !== message.id)
      : applyMessage(window.messages, message)
  const timing = { ...window.timing, completed: { ...window.timing.completed } }
  if (
    !exists &&
    message.role === 'user' &&
    !message.parts.some((part) => part.type === 'shell_command') &&
    !timing.started_at
  )
    timing.started_at = message.created_at
  const finish =
    message.role === 'assistant' && message.parts.find((part) => part.type === 'finish')?.data
  if (
    finish &&
    finish.reason !== 'tool_use' &&
    timing.started_at &&
    Number.isFinite(finish.time) &&
    timing.completed[message.id] === undefined
  ) {
    timing.completed[message.id] = Math.max(0, Math.floor(finish.time - timing.started_at))
    timing.started_at = undefined
  }
  const retained = messages.slice(-historyWindowLimit)
  timing.completed = Object.fromEntries(
    Object.entries(timing.completed).filter(([id]) => retained.some((message) => message.id === id))
  )
  return {
    ...window,
    timing,
    messages: retained,
    has_before: window.has_before || messages.length > historyWindowLimit
  }
}
