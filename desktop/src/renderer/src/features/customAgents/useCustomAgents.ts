import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type {
  BlackboardEntry,
  CreateCustomAgentInput,
  CustomAgent,
  CustomAgentEvent,
  CustomAgentRun,
  CustomAgentsRequest,
  GlobalMemoryDoc,
  ModelOption,
  ProbeResult,
  SwarmRequest,
  SwarmResult,
  UpdateCustomAgentInput
} from '../../../../shared/customAgents'
import type { DesktopEvent } from '../../../../shared/contracts'

function extractCustomAgentEvent(event: DesktopEvent): CustomAgentEvent | null {
  if ('kind' in event && event.kind === 'customAgents') {
    return event.data
  }
  if ('type' in event && event.type === 'customAgents') {
    return (event.payload as CustomAgentEvent) || null
  }
  return null
}

function cleanError(error: unknown): string {
  return String((error as Error)?.message || error).replace(
    /^Error invoking remote method '[^']+': Error: /,
    ''
  )
}

function upsertRun(runs: CustomAgentRun[], next: CustomAgentRun): CustomAgentRun[] {
  const exists = runs.some((item) => item.id === next.id)
  const merged = exists
    ? runs.map((item) => (item.id === next.id ? { ...item, ...next } : item))
    : [next, ...runs]
  return [...merged].sort((a, b) => b.started_at - a.started_at)
}

export function useCustomAgents() {
  const [agents, setAgents] = useState<CustomAgent[]>([])
  const [runs, setRuns] = useState<CustomAgentRun[]>([])
  const [blackboard, setBlackboard] = useState<BlackboardEntry[]>([])
  const [models, setModels] = useState<ModelOption[]>([])
  const [globalMemory, setGlobalMemory] = useState<GlobalMemoryDoc | null>(null)
  const [selectedAgentId, setSelectedAgentId] = useState('')
  const [selectedChainId, setSelectedChainId] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const mounted = useRef(true)

  const call = useCallback(async <T,>(request: CustomAgentsRequest): Promise<T> => {
    return (await window.prime.customAgents(request)) as T
  }, [])

  const refresh = useCallback(async () => {
    setError('')
    try {
      const [agentsRes, runsRes, boardRes, modelsRes, memoryRes] = await Promise.all([
        call<{ agents?: CustomAgent[] }>({ action: 'list' }),
        call<{ runs?: CustomAgentRun[] }>({ action: 'runs', limit: 100 }),
        call<{ entries?: BlackboardEntry[] }>({ action: 'blackboard', limit: 100 }),
        call<{ models?: ModelOption[] }>({ action: 'models' }).catch(() => ({ models: [] })),
        call<GlobalMemoryDoc>({ action: 'getGlobalMemory' }).catch(() => null)
      ])
      if (!mounted.current) return
      const nextAgents = Array.isArray(agentsRes?.agents) ? agentsRes.agents : []
      setAgents(nextAgents)
      setRuns(Array.isArray(runsRes?.runs) ? runsRes.runs : [])
      setBlackboard(Array.isArray(boardRes?.entries) ? boardRes.entries : [])
      setModels(Array.isArray(modelsRes?.models) ? modelsRes.models : [])
      if (memoryRes) setGlobalMemory(memoryRes)
      setSelectedAgentId((current) => {
        if (current && nextAgents.some((a) => a.id === current)) return current
        return nextAgents[0]?.id || ''
      })
    } catch (cause) {
      if (mounted.current) setError(cleanError(cause))
    } finally {
      if (mounted.current) setLoading(false)
    }
  }, [call])

  useEffect(() => {
    mounted.current = true
    void refresh()
    const unsubscribe = window.prime.onEvent((event) => {
      const custom = extractCustomAgentEvent(event)
      if (!custom || !mounted.current) return
      if (custom.type === 'agent.status' || custom.type === 'agent.created') {
        const updated = custom.payload as CustomAgent | undefined
        if (updated?.id) {
          setAgents((current) => {
            const exists = current.some((item) => item.id === updated.id)
            return exists
              ? current.map((item) => (item.id === updated.id ? updated : item))
              : [...current, updated]
          })
          setSelectedAgentId((current) => current || updated.id)
        }
        return
      }
      if (custom.type === 'run.step' && custom.run_id) {
        const step = custom.payload
        if (step) {
          setRuns((current) =>
            current.map((run) =>
              run.id === custom.run_id
                ? { ...run, steps: [...(run.steps || []), step] }
                : run
            )
          )
        }
        return
      }
      if (
        [
          'run.started',
          'run.verifying',
          'run.rework',
          'run.passed',
          'run.failed',
          'run.skipped',
          'run.cancelled'
        ].includes(custom.type)
      ) {
        const runPayload = custom.payload as CustomAgentRun | undefined
        if (runPayload?.id) {
          setRuns((current) => upsertRun(current, runPayload))
        }
        return
      }
      if (custom.type === 'blackboard.posted') {
        const entry = custom.payload as BlackboardEntry | undefined
        if (entry?.id) {
          setBlackboard((current) => [
            entry,
            ...current.filter((item) => item.id !== entry.id)
          ])
        }
      }
    })
    return () => {
      mounted.current = false
      unsubscribe()
    }
  }, [refresh])

  const createAgent = useCallback(
    async (input: CreateCustomAgentInput) => {
      setBusy(true)
      setError('')
      try {
        const created = await call<CustomAgent>({ action: 'create', input })
        if (!mounted.current) return created
        setAgents((current) => [...current.filter((a) => a.id !== created.id), created])
        setSelectedAgentId(created.id)
        return created
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const updateAgent = useCallback(
    async (agentId: string, patch: UpdateCustomAgentInput) => {
      setBusy(true)
      setError('')
      try {
        const updated = await call<CustomAgent>({ action: 'update', agentId, patch })
        if (!mounted.current) return updated
        setAgents((current) => current.map((a) => (a.id === updated.id ? updated : a)))
        return updated
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const deleteAgent = useCallback(
    async (agentId: string) => {
      setBusy(true)
      setError('')
      try {
        const res = await call<{ deleted: boolean }>({ action: 'delete', agentId })
        if (!mounted.current) return res
        setAgents((current) => {
          const remaining = current.filter((a) => a.id !== agentId)
          setSelectedAgentId((selected) =>
            selected === agentId ? remaining[0]?.id || '' : selected
          )
          return remaining
        })
        return res
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const runAgent = useCallback(
    async (params: {
      agentId: string
      prompt: string
      triggerReason?: string
      chainId?: string
      force?: boolean
      sync?: boolean
    }) => {
      setBusy(true)
      setError('')
      try {
        const run = await call<CustomAgentRun>({ action: 'run', ...params })
        if (!mounted.current) return run
        setRuns((current) => upsertRun(current, run))
        if (run.chain_id) setSelectedChainId(run.chain_id)
        return run
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const cancelRun = useCallback(
    async (agentId: string, runId: string) => {
      setError('')
      try {
        const res = await call<{ cancelled: boolean }>({ action: 'cancelRun', agentId, runId })
        void call<{ runs?: CustomAgentRun[] }>({ action: 'runs', limit: 100 })
          .then((data) => {
            if (mounted.current && Array.isArray(data?.runs)) setRuns(data.runs)
          })
          .catch(() => {})
        return res
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      }
    },
    [call]
  )

  const probeAgent = useCallback(
    async (agentId: string) => {
      setError('')
      try {
        const result = await call<ProbeResult>({ action: 'probe', agentId })
        void call<{ agents?: CustomAgent[] }>({ action: 'list' })
          .then((data) => {
            if (mounted.current && Array.isArray(data?.agents)) setAgents(data.agents)
          })
          .catch(() => {})
        return result
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      }
    },
    [call]
  )

  const postBlackboard = useCallback(
    async (input: {
      chain_id: string
      from_agent_id?: string
      from_agent_name?: string
      to_agent_id?: string
      topic: string
      payload: string
    }) => {
      setError('')
      try {
        const entry = await call<BlackboardEntry>({ action: 'postBlackboard', input })
        if (!mounted.current) return entry
        setBlackboard((current) => [entry, ...current.filter((item) => item.id !== entry.id)])
        return entry
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      }
    },
    [call]
  )

  const loadGlobalMemory = useCallback(async () => {
    const doc = await call<GlobalMemoryDoc>({ action: 'getGlobalMemory' })
    if (mounted.current) setGlobalMemory(doc)
    return doc
  }, [call])

  const saveGlobalMemory = useCallback(
    async (input: Partial<GlobalMemoryDoc>) => {
      setBusy(true)
      try {
        const saved = await call<GlobalMemoryDoc>({ action: 'saveGlobalMemory', input })
        if (mounted.current) setGlobalMemory(saved)
        return saved
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const dispatchSwarm = useCallback(
    async (input: SwarmRequest) => {
      setBusy(true)
      setError('')
      try {
        const result = await call<SwarmResult>({ action: 'dispatchSwarm', input })
        if (!mounted.current) return result
        const workerRuns = (result.workers || [])
          .map((worker) => worker.run)
          .filter((run): run is CustomAgentRun => Boolean(run && run.id))
        const allNewRuns = result.synthesizer_run
          ? [result.synthesizer_run, ...workerRuns]
          : workerRuns
        if (allNewRuns.length > 0) {
          setRuns((current) => allNewRuns.reduce((acc, run) => upsertRun(acc, run), current))
        }
        if (Array.isArray(result.blackboard) && result.blackboard.length > 0) {
          setBlackboard((current) => {
            const seen = new Set(result.blackboard!.map((entry) => entry.id))
            return [...result.blackboard!, ...current.filter((item) => !seen.has(item.id))]
          })
        }
        if (result.chain_id) {
          setSelectedChainId(result.chain_id)
        }
        void Promise.all([
          call<{ runs?: CustomAgentRun[] }>({ action: 'runs', limit: 100 }),
          call<{ entries?: BlackboardEntry[] }>({ action: 'blackboard', limit: 100 })
        ])
          .then(([runsRes, boardRes]) => {
            if (!mounted.current) return
            if (Array.isArray(runsRes?.runs)) setRuns(runsRes.runs)
            if (Array.isArray(boardRes?.entries)) setBlackboard(boardRes.entries)
          })
          .catch(() => {})
        return result
      } catch (cause) {
        if (mounted.current) setError(cleanError(cause))
        throw cause
      } finally {
        if (mounted.current) setBusy(false)
      }
    },
    [call]
  )

  const selectedAgent = useMemo(
    () => agents.find((item) => item.id === selectedAgentId) || null,
    [agents, selectedAgentId]
  )

  const agentRuns = useMemo(
    () => runs.filter((item) => item.agent_id === selectedAgentId),
    [runs, selectedAgentId]
  )

  return {
    agents,
    runs,
    agentRuns,
    blackboard,
    models,
    globalMemory,
    selectedAgent,
    selectedAgentId,
    setSelectedAgentId,
    selectedChainId,
    setSelectedChainId,
    loading,
    busy,
    error,
    clearError: () => setError(''),
    refresh,
    createAgent,
    updateAgent,
    deleteAgent,
    runAgent,
    cancelRun,
    probeAgent,
    postBlackboard,
    loadGlobalMemory,
    saveGlobalMemory,
    dispatchSwarm
  }
}
