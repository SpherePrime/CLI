import { useEffect, useRef } from 'react'
import { realSessionId } from '../../../../shared/pendingSession'
import type { Workspace } from '../../app/useWorkspace'

export function useBrowserView(session: Workspace['session'], inspectorOpen = false) {
  const sessionId = realSessionId(session) || null
  const viewed = useRef(sessionId)
  viewed.current = sessionId
  useEffect(() => {
    void window.prime.browserView(sessionId, inspectorOpen).catch(() => {})
  }, [sessionId, inspectorOpen])
  return viewed
}

export function isForViewedChat(eventSessionId: unknown, viewed: string | null) {
  return typeof eventSessionId !== 'string' || eventSessionId === viewed
}
