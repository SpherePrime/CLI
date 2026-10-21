import type { Message } from '../../../../shared/contracts'

export type MessageEvent = { action: string; message: Message }
export type SequencedMessageEvent = MessageEvent & { sequence: number; workspaceId: string }

export function mergeMessageAction(previous: string | undefined, next: string) {
  return previous === 'created' && next === 'updated' ? 'created' : next
}

export function queueMessageEvent<T extends MessageEvent>(
  queue: Map<string, T>,
  key: string,
  event: T,
  limit = Infinity
) {
  const action = mergeMessageAction(queue.get(key)?.action, event.action)
  queue.set(key, { ...event, action })
  while (queue.size > limit) queue.delete(queue.keys().next().value!)
}
