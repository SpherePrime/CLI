import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  Json,
  ChatReasoningMode,
  Message,
  Operation,
  Preferences,
  QueuedPrompt,
  Project,
  Session
} from '../../../shared/contracts'
import { translate } from '../i18n'
import { useAgentContext } from '../features/context/useAgentContext'
import { buildAgentPrompt } from '../../../shared/agentContext'
import { isRemoteWorkspace } from '../../../shared/remoteWorkspace'
import {
  keepPendingSessions,
  isPendingSessionId,
  newPendingSession,
  pendingSessionsOfProject
} from '../../../shared/pendingSession'
import { recordRecentModel } from '../features/models/modelPreferences'
import { useChatPermissions } from '../features/chat/useChatPermissions'
import { watchProjectSessions } from '../features/projects/projectSessionEvents'
import { useMessageHistory } from '../features/chat/useMessageHistory'
import { updateHistoryMessage, type MessagePage } from '../features/chat/messageWindow'
import { queueMessageEvent, type SequencedMessageEvent } from '../features/chat/messageEvents'
import { createResourceRefresh, resourceEvents, resourceOperations } from './resourceRefresh'
import { reconcileObservedBusy, type ObservedBusy } from './sessionBusy'

const resolvedLimit = 1000

function upsertSession(items: Session[], session: Session) {
  return [session, ...items.filter((item) => item.id !== session.id)]
}

function rememberResolved(keys: Set<string>, key: string) {
  keys.add(key)
  if (keys.size <= resolvedLimit) return
  for (const oldest of keys) {
    keys.delete(oldest)
    if (keys.size <= resolvedLimit) break
  }
}

export function useWorkspace() {
  const [info, setInfo] = useState<Json>({ runtime: 'starting' })
  const [startupReady, setStartupReady] = useState(false)
  const startupRestore = useRef<Promise<void> | null>(null)
  const [preferences, setPreferences] = useState<Preferences>({
    theme: 'dark',
    language: 'ru',
    recent: [],
    pinned: [],
    drafts: {},
    sidebarWidth: 256,
    inspectorWidth: 350
  })
  const [projects, setProjects] = useState<Project[]>([])
  const language = useRef(preferences.language)
  const preferenceWrite = useRef(0)
  language.current = preferences.language
  const [project, setProject] = useState<Project | null>(null)
  const [sessions, setSessions] = useState<Session[]>([])
  const [session, setSession] = useState<Session | null>(null)
  const [draftReasoningMode, setDraftReasoningMode] = useState<ChatReasoningMode | null>(null)
  const history = useMessageHistory()
  const { messagesSessionId } = history
  const [permissions, setPermissions] = useState<Json[]>([])
  const [questions, setQuestions] = useState<Json[]>([])
  const [config, setConfig] = useState<Json>({})
  const [agent, setAgent] = useState<Json>({})
  const [queue, setQueue] = useState<QueuedPrompt[]>([])
  const [resources, setResources] = useState<Json>({ mcp: {}, lsp: {}, skills: [] })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [retryNotice, setRetryNotice] = useState<{ sessionId: string; at: number } | null>(null)
  const [observedBusy, setObservedBusy] = useState<ObservedBusy>(null)
  const [pendingShell, setPendingShell] = useState<{ projectId: string; sessionId: string } | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const current = useRef({ project, session })
  const messageSequence = useRef(0)
  const messageEvents = useRef(new Map<string, SequencedMessageEvent>())
  const messageSnapshots = useRef(0)
  const resolvedPermissions = useRef(new Set<string>())
  const resolvedQuestions = useRef(new Set<string>())
  const requestSequence = useRef(0)
  const requestEvents = useRef<
    { sequence: number; workspaceId: string; type: string; data: any }[]
  >([])
  const requestSnapshots = useRef(0)
  current.current = { project, session }
  const sessionsRef = useRef<Session[]>([])
  sessionsRef.current = sessions
  // Chats the user asked for whose engine session is still being created, and
  // the workspace open that a personal chat is waiting for.
  const pendingSessions = useRef(new Map<string, Promise<Session | null>>())
  const pendingProjects = useRef(new Map<string, string>())
  const pendingReasoning = useRef(new Map<string, ChatReasoningMode>())
  const discardedSessions = useRef(new Set<string>())
  const pendingChatOpen = useRef<Promise<Project> | null>(null)
  // A refresh that lands while a chat is still materializing must not drop its
  // row from the sidebar, and a list built for another project must not keep it.
  const withPendingSessions = (items: Session[], projectId: string) =>
    keepPendingSessions(
      items,
      pendingSessionsOfProject(sessionsRef.current, pendingProjects.current, projectId).filter(
        (item) => !discardedSessions.current.has(item.id)
      )
    )
  useEffect(() => {
    if (!project) return
    const projectId = project.id
    return watchProjectSessions([projectId], window.prime, (id, snapshot) => {
      if (current.current.project?.id !== id) return
      const sessionId = current.current.session?.id
      if (!sessionId) return
      setSessions(withPendingSessions(snapshot, id))
      const selected = snapshot.find((item) => item.id === sessionId)
      if (selected) {
        setSession(selected)
        setObservedBusy({ projectId: id, sessionId, busy: Boolean(selected.is_busy) })
      }
    })
  }, [project?.id])
  const errorHandler = useCallback(
    (error: unknown) =>
      setError(
        String((error as Error).message || error).replace(
          /^Error invoking remote method '[^']+': Error: /,
          ''
        )
      ),
    []
  )
  const call = useCallback(async (operation: Operation, args?: Json) => {
    const id = current.current.project?.id
    if (!id) throw new Error(translate(language.current, 'Сначала откройте проект'))
    return window.prime.call(id, operation, args)
  }, [])
  const pendingFetches = useRef(new Map<string, Promise<void>>())
  const openingProject = useRef<Promise<void> | null>(null)
  const refreshPending = useCallback(async () => {
    const { project } = current.current
    if (!project) return
    const existing = pendingFetches.current.get(project.id)
    if (existing) return existing
    const requestSnapshotSequence = requestSequence.current
    const request = (async () => {
      const snapshot = await window.prime.call(project.id, 'pending')
      if (current.current.project?.id !== project.id) return
      let pendingPermissions: any[] = snapshot.permissions || []
      const q = snapshot.questions || snapshot.question
      let pendingQuestions: any[] = Array.isArray(q) ? q : q ? [q] : []
      for (const event of requestEvents.current) {
        if (event.sequence <= requestSnapshotSequence || event.workspaceId !== project.id) continue
        if (event.type === 'permission_request')
          pendingPermissions = [
            ...pendingPermissions.filter((item) => item.id !== event.data.id),
            event.data
          ]
        if (event.type === 'permission_notification' && (event.data.granted || event.data.denied))
          pendingPermissions = pendingPermissions.filter(
            (item) => item.tool_call_id !== event.data.tool_call_id
          )
        if (event.type === 'question_batch_request')
          pendingQuestions = [
            ...pendingQuestions.filter((item) => item.id !== event.data.id),
            event.data
          ]
        if (event.type === 'question_batch_notification')
          pendingQuestions = pendingQuestions.filter((item) => item.id !== event.data.batch_id)
      }
      setPermissions(
        pendingPermissions.filter(
          (item) => !resolvedPermissions.current.has(`${project.id}:${item.tool_call_id}`)
        )
      )
      setQuestions(
        pendingQuestions.filter(
          (item) => !resolvedQuestions.current.has(`${project.id}:${item.id}`)
        )
      )
    })()
    pendingFetches.current.set(project.id, request)
    requestSnapshots.current++
    try {
      await request
    } finally {
      pendingFetches.current.delete(project.id)
      if (--requestSnapshots.current === 0) requestEvents.current = []
    }
  }, [])
  const queueFetches = useRef(new Map<string, Promise<void>>())
  const refreshQueue = useCallback(async () => {
    const { project, session } = current.current
    if (!project || !session || isPendingSessionId(session.id)) return
    const key = `${project.id}:${session.id}`
    const existing = queueFetches.current.get(key)
    if (existing) return existing
    const request = (async () => {
      const entries = await window.prime.call(project.id, 'queue', { sessionId: session.id })
      if (current.current.project?.id === project.id && current.current.session?.id === session.id)
        setQueue(Array.isArray(entries) ? entries : [])
    })()
    queueFetches.current.set(key, request)
    try {
      await request
    } finally {
      queueFetches.current.delete(key)
    }
  }, [])
  const loadWorkspace = useCallback(
    async (includeMessages: boolean, heavy = true) => {
      const { project, session } = current.current
      if (!project) return
      const tasks: [string, Promise<Json>][] = [
        ['sessions', window.prime.call(project.id, 'sessions')],
        ['pending', refreshPending()],
        ['queue', refreshQueue()]
      ]
      if (heavy)
        tasks.push(
          ['config', window.prime.call(project.id, 'config')],
          ['agent', window.prime.call(project.id, 'agent')],
          ['mcp', window.prime.call(project.id, 'mcp')],
          ['lsp', window.prime.call(project.id, 'lsp')],
          ['skills', window.prime.call(project.id, 'skills')],
          ['channels', window.prime.call(project.id, 'channelMessages')]
        )
      const snapshotSequence = messageSequence.current
      const messageRequest =
        includeMessages && session && !isPendingSessionId(session.id)
          ? window.prime.call(project.id, 'messagePage', { sessionId: session.id, limit: 60 }).then(
              (value) => ({ ok: true as const, value: value as MessagePage }),
              (reason: unknown) => ({ ok: false as const, reason })
            )
          : null
      if (messageRequest) messageSnapshots.current++
      const results = await Promise.allSettled(tasks.map(([, task]) => task))
      if (current.current.project?.id !== project.id) {
        if (messageRequest && --messageSnapshots.current === 0) messageEvents.current.clear()
        return
      }
      const values: Record<string, Json> = {}
      tasks.forEach(([name], index) => {
        values[name] =
          results[index].status === 'fulfilled'
            ? (results[index] as PromiseFulfilledResult<Json>).value
            : undefined
      })
      if (values.sessions) setSessions(withPendingSessions(values.sessions, project.id))
      if (values.config) setConfig(values.config)
      if (values.agent) setAgent(values.agent)
      if (heavy)
        setResources({
          mcp: values.mcp || {},
          lsp: values.lsp || {},
          skills: values.skills || [],
          channels: values.channels || []
        })
      if (!session) return
      const active: Session[] | undefined = values.sessions
      if (messageRequest) {
        try {
          const settled = await messageRequest
          if (
            current.current.project?.id !== project.id ||
            current.current.session?.id !== session.id
          )
            return
          if (settled.ok) {
            let reconciled = settled.value
            for (const entry of messageEvents.current.values())
              if (
                entry.sequence > snapshotSequence &&
                entry.workspaceId === project.id &&
                entry.message.session_id === session.id
              )
                reconciled = updateHistoryMessage(reconciled, entry.message, entry.action)
            history.accept(session.id, reconciled)
          } else history.fail(session.id, settled.reason)
        } finally {
          if (--messageSnapshots.current === 0) messageEvents.current.clear()
        }
      }
      if (current.current.project?.id !== project.id || current.current.session?.id !== session.id)
        return
      const selected = active?.find((item: Session) => item.id === session.id)
      if (selected) {
        setSession(selected)
        setBusy(Boolean(selected.is_busy))
        setObservedBusy((observed) =>
          reconcileObservedBusy(observed, {
            projectId: project.id,
            sessionId: selected.id,
            isBusy: Boolean(selected.is_busy)
          })
        )
      }
    },
    [refreshPending, refreshQueue, history.accept]
  )
  const refreshState = useRef<{
    running: Promise<void> | null
    queued: Promise<void> | null
    queuedMessages: boolean
    queuedHeavy: boolean
  }>({ running: null, queued: null, queuedMessages: false, queuedHeavy: false })
  const scheduleRefresh = useCallback(
    (includeMessages: boolean, heavy = true): Promise<void> => {
      const state = refreshState.current
      if (!state.running) {
        state.running = loadWorkspace(includeMessages, heavy).finally(() => {
          state.running = null
        })
        return state.running
      }
      state.queuedMessages ||= includeMessages
      state.queuedHeavy ||= heavy
      if (!state.queued)
        state.queued = state.running
          .catch(() => {})
          .then(() => {
            const messages = state.queuedMessages
            const heavyQueued = state.queuedHeavy
            state.queued = null
            state.queuedMessages = false
            state.queuedHeavy = false
            return scheduleRefresh(messages, heavyQueued)
          })
      return state.queued
    },
    [loadWorkspace]
  )
  const refresh = useCallback(() => scheduleRefresh(true), [scheduleRefresh])
  const refreshLight = useCallback(() => scheduleRefresh(false, false), [scheduleRefresh])
  useEffect(() => {
    async function restoreWorkspace() {
      const saved = await window.prime.preferences()
      let opened = await window.prime.projects()
      // The shell renders as soon as the saved state is known. Restoring the
      // last project workspace is slow (the engine builds a workspace for it),
      // so it continues in the background while the window is already usable.
      setProjects(opened)
      setStartupReady(true)
      const lastProject = saved.activeProject
        ? saved.recent.includes(saved.activeProject) || isRemoteWorkspace(saved.activeProject)
          ? saved.activeProject
          : ''
        : saved.recent[0]
      if (!opened.length && lastProject) {
        const restored = await window.prime.openProject(lastProject).catch(() => null)
        if (restored) opened = [restored]
      }
      if (!opened.length) opened = [await window.prime.openChat()]
      setProjects(opened)
      if (opened.length && !current.current.project) {
        const restored = opened.find((item) => item.path === saved.activeProject) || opened[0]
        current.current.project = restored
        setProject(restored)
        const existing: Session[] = await window.prime.call(restored.id, 'sessions')
        if (existing?.length) {
          const selected =
            existing.find(
              (item) =>
                item.id === (restored.currentSessionId || saved.activeSessions?.[restored.path])
            ) || [...existing].sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0))[0]
          current.current.session = selected
          setSession(selected)
        }
        void refresh().catch(errorHandler)
      }
    }
    function restore(): Promise<void> {
      if (!startupRestore.current)
        startupRestore.current = restoreWorkspace().finally(() => {
          startupRestore.current = null
        })
      return startupRestore.current
    }
    void window.prime
      .info()
      .then(async (value) => {
        setInfo(value)
        if (value.runtime === 'ready') await restore()
      })
      .catch(errorHandler)
    void window.prime.preferences().then(setPreferences).catch(errorHandler)
    const resourceRefresh = createResourceRefresh(async (name) => {
      const id = current.current.project?.id
      if (!id) return
      const value = await window.prime.call(id, resourceOperations[name])
      if (current.current.project?.id !== id) return
      setResources((resources: Json) => ({
        ...resources,
        [name]: value || (name === 'skills' || name === 'channels' ? [] : {})
      }))
    })
    const unsubscribe = window.prime.onEvent((event) => {
      if (event.type !== 'message') history.flush()
      if (event.type === 'workbench' && event.payload?.action === 'contextChanged') {
        const { key, context } = event.payload
        if (typeof key === 'string' && context)
          setPreferences((value) => ({
            ...value,
            agentContexts: { ...value.agentContexts, [key]: context }
          }))
        return
      }
      if (event.type === 'workbench' && event.payload?.action === 'openSession') {
        void (async () => {
          const opened = await window.prime.projects()
          const targetProject = opened.find((item) => item.id === event.workspaceId)
          if (!targetProject)
            throw new Error(translate(language.current, 'Проект задачи недоступен'))
          const available: Session[] = await window.prime.call(targetProject.id, 'sessions')
          const targetSession = available.find((item) => item.id === event.payload.sessionId)
          if (!targetSession) throw new Error(translate(language.current, 'Чат задачи недоступен'))
          await window.prime.call(targetProject.id, 'currentSession', {
            sessionId: targetSession.id
          })
          current.current = { project: targetProject, session: targetSession }
          setProjects(opened)
          setProject(targetProject)
          setSession(targetSession)
          history.reset()
          setPermissions([])
          setQuestions([])
          setQueue([])
          setBusy(Boolean(targetSession.is_busy))
          await refresh()
        })().catch(errorHandler)
        return
      }
      if (event.type === 'automation') {
        if (typeof event.payload?.activeCount === 'number')
          setInfo((current: Json) => ({
            ...current,
            automationCounts: {
              activeCount: event.payload.activeCount,
              enabledCount: event.payload.enabledCount
            }
          }))
        return
      }
      if (event.type === 'projectsChanged') {
        void window.prime
          .projects()
          .then((opened) => {
            setProjects(opened)
            const active = current.current.project
            if (!active || opened.some((item) => item.id === active.id)) return
            const next = opened[0]
            current.current = { project: next ?? null, session: null }
            setProject(next ?? null)
            setSession(null)
            history.reset()
            setSessions([])
            setBusy(false)
            setPermissions([])
            setQuestions([])
            setQueue([])
            if (next) void refresh().catch(() => {})
          })
          .catch(errorHandler)
        return
      }
      if (event.type === 'terminal' && typeof event.payload?.activeCount === 'number') {
        setInfo((current: Json) =>
          current.terminalCount === event.payload.activeCount
            ? current
            : { ...current, terminalCount: event.payload.activeCount }
        )
        return
      }
      if (event.type === 'recovered') {
        setObservedBusy(null)
        const opened: Project[] = event.payload
        const restored =
          opened.find((item) => item.path === current.current.project?.path) || opened[0]
        setProjects(opened)
        if (restored) {
          current.current.project = restored
          setProject(restored)
          setBusy(false)
          setError(
            translate(
              language.current,
              'Prime был перезапущен. Незавершённые задачи остановлены; повторный запуск выполняется вручную.'
            )
          )
          void refresh()
        }
        return
      }
      if (event.type === 'runtime') {
        setInfo((value: Json) => ({
          ...value,
          runtime: event.payload.status,
          error: event.payload.error,
          runtimeVersion: event.payload.version
        }))
        if (event.payload.status === 'ready') {
          setObservedBusy(null)
          void restore().catch(errorHandler)
        }
        return
      }
      if (event.workspaceId !== current.current.project?.id) return
      const payload = event.payload?.data
      if (
        [
          'permission_request',
          'permission_notification',
          'question_batch_request',
          'question_batch_notification'
        ].includes(event.type) &&
        requestSnapshots.current > 0
      ) {
        requestEvents.current.push({
          sequence: ++requestSequence.current,
          workspaceId: event.workspaceId,
          type: event.type,
          data: payload
        })
        if (requestEvents.current.length > 2000) requestEvents.current.splice(0, 1000)
      }
      if (event.type === 'connection') {
        setProject((project) => (project ? { ...project, connection: event.payload } : null))
        if (event.payload === 'ready') void refresh()
        return
      }
      if (event.type === 'message') {
        ++messageSequence.current
        if (messageSnapshots.current > 0 && payload?.id)
          queueMessageEvent(
            messageEvents.current,
            `${event.workspaceId}:${payload.id}`,
            {
              sequence: messageSequence.current,
              workspaceId: event.workspaceId,
              action: event.payload.action,
              message: payload
            },
            2000
          )
        if (payload?.session_id === current.current.session?.id) {
          history.live(payload, event.payload.action)
          if (
            payload.role === 'assistant' &&
            event.payload.action !== 'deleted' &&
            !(payload.parts || []).some((part: any) => part.type === 'finish')
          ) {
            setBusy(true)
            setObservedBusy({
              projectId: event.workspaceId,
              sessionId: payload.session_id,
              busy: true
            })
          }
        }
      }
      if (event.type === 'session') {
        setSessions((items) => {
          if (event.payload.action === 'deleted')
            return items.filter((item) => item.id !== payload.id)
          if (event.payload.action === 'created') return upsertSession(items, payload)
          return items.map((item) => (item.id === payload.id ? { ...item, ...payload } : item))
        })
        if (payload?.id === current.current.session?.id)
          setSession((previous: any) => ({ ...previous, ...payload }))
      }
      if (event.type === 'run_complete') {
        setRetryNotice((notice) => (notice?.sessionId === payload.session_id ? null : notice))
        if (payload.session_id === current.current.session?.id) {
          setBusy(false)
          setObservedBusy({
            projectId: event.workspaceId,
            sessionId: payload.session_id,
            busy: false
          })
          if (payload.error && !payload.cancelled && !/context canceled/i.test(payload.error))
            setError(payload.error)
        }
        void refreshLight()
      }
      if (event.type === 'agent_event' && payload?.type === 'provider_retry') {
        setRetryNotice({ sessionId: payload.session_id, at: Date.now() })
        return
      }
      if (event.type === 'agent_event') {
        if (payload?.type === 'error' && !/context canceled/i.test(payload.error || ''))
          setError(payload.error || translate(language.current, 'Ошибка агента'))
        void refreshLight()
      }
      if (event.type === 'permission_request')
        setPermissions((items) => [...items.filter((item) => item.id !== payload.id), payload])
      if (event.type === 'permission_notification' && !payload.granted && !payload.denied)
        void refreshPending().catch(() => {})
      if (event.type === 'permission_notification' && (payload.granted || payload.denied)) {
        rememberResolved(
          resolvedPermissions.current,
          `${event.workspaceId}:${payload.tool_call_id}`
        )
        setPermissions((items) =>
          items.filter((item) => item.tool_call_id !== payload.tool_call_id)
        )
      }
      if (event.type === 'question_batch_request')
        setQuestions((items) => [...items.filter((item) => item.id !== payload.id), payload])
      if (event.type === 'question_batch_notification') {
        rememberResolved(resolvedQuestions.current, `${event.workspaceId}:${payload.batch_id}`)
        setQuestions((items) => items.filter((item) => item.id !== payload.batch_id))
      }
      if (event.type === 'config_changed') void scheduleRefresh(false, true).catch(() => {})
      const resource = resourceEvents[event.type]
      if (resource) resourceRefresh.schedule(resource)
    })
    return () => {
      resourceRefresh.dispose()
      unsubscribe()
    }
  }, [refresh, scheduleRefresh, refreshPending, errorHandler])
  const hasPending = permissions.length > 0 || questions.length > 0
  useEffect(() => {
    if (!project || (!busy && !hasPending)) return
    const sync = () => {
      void refreshPending().catch(() => {})
      void refreshQueue().catch(() => {})
    }
    sync()
    const timer = setInterval(sync, 1000)
    return () => clearInterval(timer)
  }, [project?.id, session?.id, busy, hasPending, refreshPending, refreshQueue])
  useEffect(() => {
    document.documentElement.dataset.theme =
      preferences.theme === 'system'
        ? matchMedia('(prefers-color-scheme: dark)').matches
          ? 'dark'
          : 'light'
        : preferences.theme
  }, [preferences.theme])
  useEffect(() => {
    document.documentElement.dataset.density = preferences.density || 'comfortable'
    document.documentElement.dataset.motion = preferences.reduceMotion ? 'reduced' : 'full'
  }, [preferences.density, preferences.reduceMotion])
  useEffect(() => {
    if (!project) return
    const timer = setInterval(() => {
      if (document.hidden) return
      void scheduleRefresh(current.current.project?.connection !== 'ready', false).catch(() => {})
    }, 10000)
    return () => clearInterval(timer)
  }, [project?.id, scheduleRefresh])
  async function openProject(path?: string) {
    await openWorkspace(() => window.prime.openProject(path))
  }
  async function openProjectSession(path: string, selected: Session, known?: Session[]) {
    await openProject(path)
    const target = current.current.project
    if (!target || target.path !== path) return
    await openSession(target, selected, known)
  }
  async function openProjectNewChat(path: string) {
    await openProject(path)
    await createSession()
  }
  async function openRemote(profileId: string) {
    await openWorkspace(() => window.prime.openRemote(profileId))
  }
  // Waits until a project is available. The startup restore and the open
  // dialog both run in the background, so a prompt typed while the workspace
  // is still being built is queued here instead of being rejected.
  async function waitForProject() {
    if (current.current.project) return
    await (openingProject.current ?? startupRestore.current ?? pendingChatOpen.current)?.catch(
      () => {}
    )
  }
  async function openWorkspace(open: () => Promise<Project | null>) {
    setLoading(true)
    setError('')
    const task = (async () => {
      const opened = await open()
      if (!opened) return
      current.current = { project: opened, session: null }
      setProject(opened)
      setSession(null)
      history.reset()
      setPermissions([])
      setQuestions([])
      setBusy(false)
      setQueue([])
      setLoading(false)
      void (async () => {
        void window.prime.projects().then(setProjects).catch(errorHandler)
        void window.prime.preferences().then(setPreferences).catch(errorHandler)
        await refresh()
      })().catch(errorHandler)
    })()
    openingProject.current = task
    try {
      await task
    } catch (error) {
      errorHandler(error)
    } finally {
      if (openingProject.current === task) openingProject.current = null
      setLoading(false)
    }
  }
  async function selectProject(selected: Project) {
    setDraftReasoningMode(null)
    current.current = { project: selected, session: null }
    setProject(selected)
    setSession(null)
    history.reset()
    setBusy(false)
    setQueue([])
    void refreshLight().catch(errorHandler)
    void scheduleRefresh(false, true).catch(() => {})
  }
  async function deleteProject(path: string) {
    try {
      const result = (await window.prime.deleteProject(path)) as unknown as {
        warnings?: string[]
      } | null
      const saved = await window.prime.preferences()
      setPreferences(saved)
      const remaining = projects.filter((item) => item.path !== path)
      setProjects(remaining)
      if (current.current.project?.path === path) {
        current.current = { project: null, session: null }
        setProject(null)
        setSession(null)
        history.reset()
        setSessions([])
        setBusy(false)
        setQueue([])
        setPermissions([])
        setQuestions([])
        setResources({ mcp: {}, lsp: {}, skills: [] })
        if (remaining[0]) await selectProject(remaining[0])
      }
      return { warnings: result?.warnings || [] }
    } catch (error) {
      errorHandler(error)
      return { warnings: [] }
    }
  }
  // Shows a chat immediately: a cached page is reused, otherwise the message
  // page is requested right away instead of waiting for the rest of the
  // workspace state. The remaining state loads afterwards in the background.
  function showSession(project: Project, selected: Session, known?: Session[]) {
    // A chat the engine has not created yet has no page to load.
    if (!history.select(selected.id) && !isPendingSessionId(selected.id))
      void history.load(project.id, selected.id, 'latest')
    current.current = { project, session: selected }
    setProject(project)
    setSession(selected)
    if (known) setSessions(known)
    setBusy(Boolean(selected.is_busy))
    setQueue([])
  }
  async function selectSession(selected: Session) {
    const project = current.current.project
    if (!project) return
    setDraftReasoningMode(null)
    setPermissions([])
    setQuestions([])
    showSession(project, selected)
    if (isPendingSessionId(selected.id)) return
    void call('currentSession', { sessionId: selected.id }).catch(errorHandler)
    void scheduleRefresh(true, false).catch(errorHandler)
  }
  async function openSession(target: Project, selected: Session, known?: Session[]) {
    if (target.id === current.current.project?.id) return selectSession(selected)
    setDraftReasoningMode(null)
    setPermissions([])
    setQuestions([])
    showSession(target, selected, known)
    if (isPendingSessionId(selected.id)) return
    void window.prime
      .call(target.id, 'currentSession', { sessionId: selected.id })
      .catch(errorHandler)
    void refresh().catch(errorHandler)
  }
  async function openChat() {
    try {
      const opened = await window.prime.openChat()
      void window.prime.projects().then(setProjects).catch(errorHandler)
      await selectProject(opened)
      const [existing, saved] = await Promise.all([
        window.prime.call(opened.id, 'sessions') as Promise<Session[]>,
        window.prime.preferences()
      ])
      const selected =
        existing.find(
          (item) => item.id === (opened.currentSessionId || saved.activeSessions?.[opened.path])
        ) || [...existing].sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0))[0]
      if (selected) await selectSession(selected)
    } catch (error) {
      errorHandler(error)
    }
  }
  // Creates the chat in the engine and swaps the placeholder the UI already
  // shows for the real session. A chat the user discarded meanwhile is removed
  // again instead of appearing in the sidebar.
  async function materializeSession(
    project: Project,
    placeholder: Session
  ): Promise<Session | null> {
    try {
      const created = (await window.prime.call(project.id, 'createSession', {
        title: translate(language.current, 'Новый чат')
      })) as Session
      if (discardedSessions.current.delete(placeholder.id)) {
        void window.prime
          .call(project.id, 'deleteSession', { sessionId: created.id })
          .catch(() => {})
        return null
      }
      // The list belongs to the open project, so a chat created for a project
      // the user has left meanwhile is not inserted into another project.
      if (current.current.project?.id === project.id)
        setSessions((items) =>
          upsertSession(
            items.filter((item) => item.id !== placeholder.id),
            created
          )
        )
      if (current.current.session?.id === placeholder.id) {
        current.current = { project, session: created }
        setSession(created)
        history.select(created.id)
        void history.load(project.id, created.id, 'latest')
        void window.prime
          .call(project.id, 'currentSession', { sessionId: created.id })
          .catch(errorHandler)
        void scheduleRefresh(false, false).catch(() => {})
      }
      const reasoning = pendingReasoning.current.get(placeholder.id)
      void savePreferences({
        reasoningBySession: { [created.id]: reasoning || draftReasoningMode || reasoningFallback }
      }).catch(errorHandler)
      return created
    } catch (error) {
      setSessions((items) => items.filter((item) => item.id !== placeholder.id))
      if (current.current.session?.id === placeholder.id) {
        current.current = { project, session: null }
        setSession(null)
        history.reset()
      }
      errorHandler(error)
      return null
    }
  }
  // Shows the new chat at once and finishes the engine work in the background.
  function startSession(project: Project, placeholder: Session) {
    setSessions((items) => upsertSession(items, placeholder))
    setPermissions([])
    setQuestions([])
    showSession(project, placeholder)
    setDraftReasoningMode(null)
    const task = materializeSession(project, placeholder)
    pendingSessions.current.set(placeholder.id, task)
    pendingProjects.current.set(placeholder.id, project.id)
    void task.finally(() => {
      pendingSessions.current.delete(placeholder.id)
      pendingProjects.current.delete(placeholder.id)
      pendingReasoning.current.delete(placeholder.id)
      discardedSessions.current.delete(placeholder.id)
    })
    return placeholder
  }
  // A personal chat needs the chats workspace, which may still be closed. The
  // chat appears as soon as the workspace answers; a prompt typed meanwhile
  // waits for it instead of being rejected.
  async function createPersonalChat(): Promise<Session | null> {
    const opening = (async () => {
      const opened = await window.prime.openChat()
      void window.prime.projects().then(setProjects).catch(errorHandler)
      if (current.current.project?.id !== opened.id) await selectProject(opened)
      return opened
    })()
    pendingChatOpen.current = opening
    try {
      return startSession(
        await opening,
        newPendingSession(translate(language.current, 'Новый чат'))
      )
    } catch (error) {
      errorHandler(error)
      return null
    } finally {
      if (pendingChatOpen.current === opening) pendingChatOpen.current = null
    }
  }
  async function createSession() {
    const target = current.current.project ?? projects.find((item) => item.kind === 'chat') ?? null
    if (!target) return createPersonalChat()
    return startSession(target, newPendingSession(translate(language.current, 'Новый чат')))
  }
  async function send(prompt: string, attachmentIds: string[] = []) {
    if (!prompt.trim()) return false
    await waitForProject()
    // A chat the user just asked for may still be materializing: wait for its
    // real session instead of sending into the placeholder id. The prompt stays
    // in the chat it was typed in even when the user switched meanwhile.
    const placeholder = current.current.session
    const placeholderProject = current.current.project
    let created: Session | null = null
    if (placeholder && isPendingSessionId(placeholder.id)) {
      created = (await pendingSessions.current.get(placeholder.id)?.catch(() => null)) ?? null
      if (!created) return false
    }
    const targetProject = created ? placeholderProject : current.current.project
    if (!targetProject) return false
    const targetSession = created ?? current.current.session
    const usedModel =
      (targetSession && config.session_models?.[targetSession.id]) ?? config.models?.large
    try {
      const latestPreferences = await window.prime.preferences()
      setPreferences(latestPreferences)
      const fallbackReasoning = latestPreferences.lastReasoningMode || 'off'
      const reasoningMode = targetSession
        ? latestPreferences.reasoningBySession?.[targetSession.id] || fallbackReasoning
        : draftReasoningMode || fallbackReasoning
      const latestContext =
        latestPreferences.agentContexts?.[targetProject.path] ?? agentContext.context
      const preparedPrompt = await buildAgentPrompt(prompt, latestContext, (path) =>
        window.prime.readFile(targetProject.id, path)
      )
      let selected = targetSession
      if (!selected) {
        selected = await window.prime.call(targetProject.id, 'createSession', {
          title: translate(language.current, 'Новый чат')
        })
      }
      if (!selected) return false
      if (!targetSession) {
        await savePreferences({ reasoningBySession: { [selected.id]: reasoningMode } })
      }
      const runId = crypto.randomUUID()
      await chatPermissions.prepareSession(targetProject.id, selected.id, chatPermissions.skip)
      if (
        current.current.project?.id === targetProject.id &&
        current.current.session?.id === selected.id
      ) {
        setObservedBusy(null)
        setBusy(true)
      }
      if (prompt.startsWith('!'))
        setPendingShell({ projectId: targetProject.id, sessionId: selected.id })
      if (prompt.startsWith('!'))
        await window.prime.call(targetProject.id, 'shell', {
          sessionId: selected.id,
          command: prompt.slice(1).trim()
        })
      else {
        await window.prime
          .call(targetProject.id, 'mode', { mode: latestPreferences.agentMode || 'general' })
          .catch(() => {})
        await window.prime.call(targetProject.id, 'send', {
          sessionId: selected.id,
          runId,
          prompt: preparedPrompt,
          reasoningMode,
          attachmentIds
        })
        if (usedModel?.provider && usedModel?.model) {
          try {
            const latest = await window.prime.preferences()
            await savePreferences({
              recentModels: recordRecentModel(latest.recentModels, usedModel)
            })
          } catch (error) {
            errorHandler(error)
          }
        }
      }
      if (current.current.project?.id === targetProject.id && !current.current.session) {
        setSessions((items) => upsertSession(items, selected!))
        await selectSession(selected)
      }
      void refreshLight().catch(errorHandler)
      return true
    } catch (error) {
      errorHandler(error)
      void refreshLight().catch(errorHandler)
      return false
    } finally {
      if (prompt.startsWith('!')) setPendingShell(null)
    }
  }
  async function savePreferences(patch: Partial<Preferences>) {
    const sequence = ++preferenceWrite.current
    setPreferences((current) => ({
      ...current,
      ...patch,
      drafts: { ...current.drafts, ...patch.drafts },
      skipBySession: { ...current.skipBySession, ...patch.skipBySession },
      reasoningBySession: { ...current.reasoningBySession, ...patch.reasoningBySession },
      agentContexts: { ...current.agentContexts, ...patch.agentContexts }
    }))
    try {
      const saved = await window.prime.preferences(patch)
      if (sequence === preferenceWrite.current) setPreferences(saved)
    } catch (error) {
      if (sequence === preferenceWrite.current) setPreferences(await window.prime.preferences())
      throw error
    }
  }
  const agentContext = useAgentContext(project, preferences, savePreferences, errorHandler)
  const chatPermissions = useChatPermissions({
    projectId: project?.id,
    sessionId: session && !isPendingSessionId(session.id) ? session.id : undefined,
    preferences,
    savePreferences,
    onError: errorHandler
  })
  const reasoningFallback = preferences.lastReasoningMode || 'off'
  const reasoningMode =
    session && !isPendingSessionId(session.id)
      ? preferences.reasoningBySession?.[session.id] || reasoningFallback
      : draftReasoningMode || reasoningFallback
  async function setReasoningMode(mode: ChatReasoningMode) {
    const selected = current.current.session
    const materialized = selected && !isPendingSessionId(selected.id) ? selected : null
    if (!materialized) {
      // The chat may still be materializing, so the choice is remembered for
      // the session the engine is about to create.
      if (selected) pendingReasoning.current.set(selected.id, mode)
      setDraftReasoningMode(mode)
    }
    await savePreferences({
      lastReasoningMode: mode,
      ...(materialized ? { reasoningBySession: { [materialized.id]: mode } } : {})
    })
  }
  async function deleteSession(id: string) {
    if (isPendingSessionId(id)) {
      // The engine has not created this chat yet; the background creation
      // removes the session it makes instead of showing it. Forgetting the
      // placeholder keeps a refresh from putting its row back.
      discardedSessions.current.add(id)
      pendingSessions.current.delete(id)
      pendingProjects.current.delete(id)
    } else {
      await call('deleteSession', { sessionId: id })
    }
    history.delete(id)
    setSessions((items) => items.filter((item) => item.id !== id))
    if (current.current.session?.id === id) {
      current.current.session = null
      setSession(null)
      history.reset()
      setBusy(false)
      setQueue([])
    }
    void refreshLight().catch(errorHandler)
  }
  return {
    retrying: Boolean(retryNotice && retryNotice.sessionId === session?.id),
    agentContext: agentContext.context,
    updateAgentContext: agentContext.updateContext,
    info,
    startupReady,
    preferences,
    savePreferences,
    skipPermissions: chatPermissions.skip,
    setPermissionMode: chatPermissions.setSkip,
    reasoningMode,
    setReasoningMode,
    projects,
    project,
    sessions,
    session,
    get messages() {
      return history.getPage().messages
    },
    history,
    loadHistory: (direction: 'before' | 'after' | 'latest') => {
      const { project, session } = current.current
      return project && session
        ? history.load(project.id, session.id, direction)
        : Promise.resolve()
    },
    searchMessages: useCallback(
      (query: string) => {
        const id = current.current.session?.id
        return id && !isPendingSessionId(id)
          ? call('messageSearch', { sessionId: id, query })
          : Promise.resolve([] as Message[])
      },
      [call]
    ),
    revealMessage: useCallback(
      (id: string) => {
        const { project, session } = current.current
        return project && session ? history.reveal(project.id, session.id, id) : Promise.resolve()
      },
      [history.reveal]
    ),
    messagesReady:
      !session || isPendingSessionId(session.id) || messagesSessionId === session.id,
    permissions,
    questions,
    config,
    agent,
    resources,
    queue,
    refreshQueue,
    error,
    setError,
    errorHandler,
    loading,
    busy:
      (observedBusy &&
      observedBusy.projectId === project?.id &&
      observedBusy.sessionId === session?.id
        ? observedBusy.busy
        : busy) ||
      Boolean(
        pendingShell &&
        pendingShell.projectId === project?.id &&
        (!session || session.id === pendingShell.sessionId)
      ),
    activeSessionId:
      session?.id ||
      (pendingShell?.projectId === project?.id ? pendingShell?.sessionId : undefined),
    setBusy,
    call,
    refresh,
    openChat,
    newPersonalChat: createPersonalChat,
    openProject,
    openProjectSession,
    openProjectNewChat,
    openRemote,
    selectProject,
    deleteProject,
    selectSession,
    openSession,
    createSession,
    deleteSession,
    send
  }
}
export type Workspace = ReturnType<typeof useWorkspace>
