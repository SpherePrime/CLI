import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  AppUiRequest,
  AppUiResponse,
  AppUiSnapshot,
  AppUiState,
  AppUiTarget
} from '../../../../shared/appTesting'

const TREE_REFRESH_DELAY = 700
const PASSIVE_ACTIONS = new Set(['snapshot', 'pick', 'check', 'audit'])

export function useAppTesting(workspaceId: string, sessionId: string | undefined, onError: (error: unknown) => void) {
  const [state, setState] = useState<AppUiState>({})
  const [windows, setWindows] = useState<AppUiTarget[]>([])
  const [snapshot, setSnapshot] = useState<AppUiSnapshot | null>(null)
  const [treeOpen, setTreeOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const active = useRef(workspaceId)
  active.current = workspaceId
  const treeTimer = useRef<number | undefined>(undefined)

  const call = useCallback(
    (request: AppUiRequest): Promise<AppUiResponse> => window.prime.appTesting(workspaceId, request),
    [workspaceId]
  )

  const apply = useCallback((result: AppUiResponse) => {
    setState((current) => ({
      target: result.target !== undefined ? result.target : current.target,
      log: result.log ?? current.log,
      launched: result.launched !== undefined ? result.launched : current.launched,
      report: result.report ?? current.report
    }))
    if (result.windows) setWindows(result.windows)
    if (result.snapshot) setSnapshot(result.snapshot)
  }, [])

  const run = useCallback(
    async (request: AppUiRequest) => {
      setBusy(true)
      try {
        const result = await call(request)
        if (active.current === workspaceId) apply(result)
      } catch (cause) {
        onError(cause)
      } finally {
        setBusy(false)
      }
    },
    [apply, call, onError, workspaceId]
  )

  const loadTree = useCallback(async () => {
    if (!sessionId) return
    try {
      const result = await call({ action: 'snapshot', sessionId })
      if (active.current === workspaceId && result.snapshot) setSnapshot(result.snapshot)
    } catch {
      setSnapshot(null)
    }
  }, [call, sessionId, workspaceId])

  useEffect(() => {
    setState({})
    setWindows([])
    setSnapshot(null)
    void run({ action: 'state' })
    return window.prime.onEvent((event) => {
      if (event.type !== 'appui' || event.workspaceId !== workspaceId || !event.payload) return
      if (event.payload.activate) return
      apply(event.payload as AppUiResponse)
    })
  }, [apply, run, workspaceId])

  const attached = !!state.target
  const lastAction = state.log?.[state.log.length - 1]

  useEffect(() => {
    if (!attached) {
      setSnapshot(null)
      return
    }
    if (!treeOpen) return
    if (lastAction && PASSIVE_ACTIONS.has(lastAction.action) && snapshot) return
    window.clearTimeout(treeTimer.current)
    treeTimer.current = window.setTimeout(() => void loadTree(), TREE_REFRESH_DELAY)
    return () => window.clearTimeout(treeTimer.current)
  }, [attached, treeOpen, lastAction?.time, lastAction?.action, state.log?.length, loadTree])

  return {
    state,
    windows,
    snapshot,
    busy,
    treeOpen,
    setTreeOpen,
    refreshWindows: () => run({ action: 'targets' }),
    refreshTree: loadTree,
    attach: (target: AppUiTarget) =>
      sessionId
        ? run({
            action: 'attach',
            sessionId,
            hwnd: target.hwnd,
            pid: target.pid,
            identity: target.identity || '',
            title: target.title,
            mode: 'control'
          })
        : Promise.resolve(),
    detach: () => run({ action: 'detach' }),
    clearReport: () => run({ action: 'clearReport' })
  }
}
