import type { Message } from '../../../../shared/contracts'

function sameValue(left: unknown, right: unknown): boolean {
  if (left === right) return true
  if (typeof left !== 'object' || typeof right !== 'object' || !left || !right) return false
  if (Array.isArray(left) !== Array.isArray(right)) return false
  if (Array.isArray(left)) {
    const other = right as unknown[]
    return left.length === other.length && left.every((item, index) => sameValue(item, other[index]))
  }
  const leftKeys = Object.keys(left)
  const rightRecord = right as Record<string, unknown>
  if (leftKeys.length !== Object.keys(rightRecord).length) return false
  return leftKeys.every(
    (key) => key in rightRecord && sameValue((left as Record<string, unknown>)[key], rightRecord[key])
  )
}

export function reuseUnchanged(previous: Message[], next: Message[]): Message[] {
  if (!previous.length) return next
  const known = new Map(previous.map((message) => [message.id, message]))
  let changed = previous.length !== next.length
  const result = next.map((message, index) => {
    const existing = known.get(message.id)
    const reused = existing && sameValue(existing, message) ? existing : message
    if (reused !== previous[index]) changed = true
    return reused
  })
  return changed ? result : previous
}
