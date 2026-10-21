import type { Session } from '../../../../shared/contracts'

type SessionSource = {
  call: (projectId: string, operation: 'sessions') => Promise<Session[]>
  onEvent: (listener: (event: any) => void) => () => void
}

export function watchProjectSessions(
  projectIds: string[],
  source: SessionSource,
  onSnapshot: (projectId: string, sessions: Session[]) => void
) {
  let disposed = false
  const states = new Map(
    projectIds.map((id) => [
      id,
      {
        loading: false,
        pending: false,
        sessions: [] as Session[],
        timer: undefined as ReturnType<typeof setTimeout> | undefined
      }
    ])
  )
  async function refresh(id: string) {
    const state = states.get(id)!
    if (disposed) return
    if (state.loading) {
      state.pending = true
      return
    }
    state.loading = true
    try {
      const sessions = await source.call(id, 'sessions')
      if (!disposed) {
        state.sessions = sessions
        onSnapshot(id, sessions)
      }
    } catch {
    } finally {
      state.loading = false
      if (state.pending && !disposed) {
        state.pending = false
        schedule(id)
      }
    }
  }
  function schedule(id: string) {
    const state = states.get(id)!
    if (state.timer || disposed) return
    state.timer = setTimeout(() => {
      state.timer = undefined
      void refresh(id)
    }, 150)
  }
  const unsubscribe = source.onEvent((event) => {
    if (event.type === 'recovered') {
      for (const id of states.keys()) schedule(id)
      return
    }
    if (!states.has(event.workspaceId)) return
    if (event.type === 'message') {
      const sessionId = event.payload?.data?.session_id
      if (
        sessionId &&
        !states
          .get(event.workspaceId)!
          .sessions.some((session) => session.id === sessionId && session.is_busy)
      )
        schedule(event.workspaceId)
      return
    }
    if (
      ['session', 'agent_event', 'run_complete'].includes(event.type) ||
      (event.type === 'connection' && event.payload === 'ready')
    )
      schedule(event.workspaceId)
  })
  for (const id of projectIds) void refresh(id)
  return () => {
    disposed = true
    unsubscribe()
    for (const state of states.values()) clearTimeout(state.timer)
  }
}
