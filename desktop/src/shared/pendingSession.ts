import type { Session } from './contracts'

// A chat the user just asked for is shown before the engine has created it, so
// the sidebar row and the composer react immediately. The placeholder id is
// replaced by the real session as soon as the engine answers.
export const pendingSessionPrefix = 'pending:'

export function isPendingSessionId(id: string | null | undefined): boolean {
  return Boolean(id && id.startsWith(pendingSessionPrefix))
}

// The engine only knows real session ids, so a caller asks for the id it may
// send and gets nothing while the chat is still a placeholder.
export function realSessionId(session: Session | null | undefined): string | undefined {
  return session && !isPendingSessionId(session.id) ? session.id : undefined
}

export function newPendingSession(title: string): Session {
  const now = Math.floor(Date.now() / 1000)
  return {
    id: `${pendingSessionPrefix}${crypto.randomUUID()}`,
    title,
    created_at: now,
    updated_at: now
  }
}

// A refresh that lands while a chat is still materializing must not drop its
// row from the sidebar, so the pending rows of the previous list are kept.
export function keepPendingSessions(items: Session[], previous: Session[]): Session[] {
  const pending = previous.filter((item) => isPendingSessionId(item.id))
  if (!pending.length) return items
  const known = new Set(items.map((item) => item.id))
  return [...pending.filter((item) => !known.has(item.id)), ...items]
}

// A pending row belongs to the project that asked for it, so a list built for
// another project never inherits it.
export function pendingSessionsOfProject(
  previous: Session[],
  projectOf: ReadonlyMap<string, string>,
  projectId: string
): Session[] {
  return previous.filter(
    (item) => isPendingSessionId(item.id) && projectOf.get(item.id) === projectId
  )
}
