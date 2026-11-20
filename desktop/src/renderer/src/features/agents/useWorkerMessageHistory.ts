import { useCallback, useEffect, useRef, useState } from 'react'
import { workerActive, type AgentWorker } from '../../../../shared/agents'
import type { PageDirection } from '../chat/messageWindow'
import { createAgentPolling } from './agentPolling'
import {
  createWorkerMessageHistory,
  type WorkerMessageHistoryState
} from './workerMessageHistory'

const initialState = (): WorkerMessageHistoryState => ({
  messages: [],
  hasBefore: false,
  hasAfter: false,
  timing: { completed: {} },
  initialLoading: true,
  pagingLoading: null,
  error: '',
  pagingError: ''
})

export function useWorkerMessageHistory({
  projectId,
  sessionId,
  worker
}: {
  projectId: string
  sessionId: string
  worker: AgentWorker
}) {
  const [state, setState] = useState<WorkerMessageHistoryState>(initialState)
  const workerRef = useRef(worker)
  workerRef.current = worker
  const intervalRef = useRef(3000)
  intervalRef.current = workerActive(worker) ? 1000 : 3000
  const historyRef = useRef<ReturnType<typeof createWorkerMessageHistory> | null>(null)
  const pollingRef = useRef<ReturnType<typeof createAgentPolling<void>> | null>(null)

  useEffect(() => {
    const history = createWorkerMessageHistory({
      fetchPage: (args) =>
        window.prime.call(projectId, 'agentWorkerMessages', {
          sessionId,
          workerId: worker.id,
          ...args
        }),
      onChange: setState
    })
    historyRef.current = history
    setState(history.getState())

    const polling = createAgentPolling({
      load: () =>
        history.poll(
          `${workerRef.current.status}:${workerRef.current.updated_at}`,
          workerActive(workerRef.current)
        ),
      receive: () => {},
      failed: () => {},
      interval: () => intervalRef.current,
      hidden: () => document.hidden
    })
    pollingRef.current = polling

    const visible = () => {
      if (!document.hidden) polling.refresh()
    }
    document.addEventListener('visibilitychange', visible)
    polling.refresh()

    return () => {
      polling.dispose()
      history.dispose()
      historyRef.current = null
      pollingRef.current = null
      document.removeEventListener('visibilitychange', visible)
    }
  }, [projectId, sessionId, worker.id])

  const loadHistory = useCallback((direction: PageDirection) => {
    return historyRef.current?.load(direction) ?? Promise.resolve()
  }, [])

  const refresh = useCallback(() => {
    historyRef.current?.resetRevision()
    pollingRef.current?.refresh()
  }, [])

  return {
    ...state,
    loadHistory,
    refresh
  }
}
