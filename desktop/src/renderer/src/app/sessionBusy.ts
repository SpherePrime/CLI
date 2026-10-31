export type ObservedBusy = {
  projectId: string
  sessionId: string
  busy: boolean
} | null

export type SessionBusySnapshot = {
  projectId: string
  sessionId: string
  isBusy: boolean
}

export function reconcileObservedBusy(
  observed: ObservedBusy,
  snapshot: SessionBusySnapshot
): ObservedBusy {
  if (!observed) return null
  if (observed.projectId !== snapshot.projectId || observed.sessionId !== snapshot.sessionId)
    return observed
  return snapshot.isBusy ? observed : null
}
