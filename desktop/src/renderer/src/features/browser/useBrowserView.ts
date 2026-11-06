import { useEffect, useRef } from 'react'
import { realSessionId } from '../../../../shared/pendingSession'
import type { Workspace } from '../../app/useWorkspace'

export function useBrowserView(session: Workspace['session']) {
  const sessionId = realSessionId(session) || null
  const viewed = useRef(sessionId)
  viewed.current = sessionId
  useEffect(() => {
    void window.prime.browserView(sessionId).catch(() => {})
  }, [sessionId])
  return viewed
}

export function isForViewedChat(eventSessionId: unknown, viewed: string | null) {
  return typeof eventSessionId !== 'string' || eventSessionId === viewed
}
