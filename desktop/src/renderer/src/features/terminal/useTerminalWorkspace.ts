import { useEffect, useRef, useState } from 'react'
import type { DesktopEvent } from '../../../../shared/contracts'
import type { TerminalApi, TerminalSnapshot } from '../../../../shared/terminal'

export type TerminalSubscribe = (listener: (event: DesktopEvent) => void) => () => void

export function useTerminalWorkspace(
  workspaceId: string,
  api: TerminalApi,
  subscribe: TerminalSubscribe,
  onError: (error: unknown) => void
) {
  const [terminals, setTerminals] = useState<TerminalSnapshot[]>([])
  const [loading, setLoading] = useState(true)
  const callbacks = useRef({ api, subscribe, onError })
  callbacks.current = { api, subscribe, onError }

  useEffect(() => {
    let disposed = false
    const latest = new Map<string, TerminalSnapshot>()
    const publish = () => {
      if (!disposed)
        setTerminals(
          [...latest.values()]
            .filter((terminal) => terminal.status !== 'closed')
            .sort((left, right) => left.createdAt - right.createdAt)
        )
    }
    const update = (terminal: TerminalSnapshot) => {
      const previous = latest.get(terminal.id)
      if (!previous || terminal.sequence >= previous.sequence) latest.set(terminal.id, terminal)
    }
    const unsubscribe = callbacks.current.subscribe((event) => {
      if (
        disposed ||
        event.workspaceId !== workspaceId ||
        event.type !== 'terminal' ||
        event.payload?.action !== 'state'
      )
        return
      update(event.payload.terminal as TerminalSnapshot)
      publish()
    })
    void callbacks.current
      .api({ action: 'list' })
      .then((response) => {
        if (disposed) return
        if (!Array.isArray(response)) throw new Error('Invalid terminal list response')
        response.forEach(update)
        publish()
      })
      .catch((error) => {
        if (!disposed) callbacks.current.onError(error)
      })
      .finally(() => {
        if (!disposed) setLoading(false)
      })
    return () => {
      disposed = true
      unsubscribe()
    }
  }, [workspaceId])

  return { terminals, loading }
}
