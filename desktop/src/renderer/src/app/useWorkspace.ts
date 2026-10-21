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
import { recordRecentModel } from '../features/models/modelPreferences'
import { useChatPermissions } from '../features/chat/useChatPermissions'
import { watchProjectSessions } from '../features/projects/projectSessionEvents'
import { useMessageHistory } from '../features/chat/useMessageHistory'
import { updateHistoryMessage, type MessagePage } from '../features/chat/messageWindow'

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
  const { messages, messagesSessionId } = history
  const [permissions, setPermissions] = useState<Json[]>([])
  const [questions, setQuestions] = useState<Json[]>([])
  const [config, setConfig] = useState<Json>({})
  const [agent, setAgent] = useState<Json>({})
  const [queue, setQueue] = useState<QueuedPrompt[]>([])
  const [resources, setResources] = useState<Json>({ mcp: {}, lsp: {}, skills: [] })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [observedBusy, setObservedBusy] = useState<{
    projectId: string
    sessionId: string
    busy: boolean
  } | null>(null)
  const [pendingShell, setPendingShell] = useState<{ projectId: string; sessionId: string } | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const current = useRef({ project, session })
  const messageSequence = useRef(0)
  const messageEvents = useRef<
    { sequence: number; workspaceId: string; action: string; message: Message }[]
  >([])
  const messageSnapshots = useRef(0)
  const resolvedPermissions = useRef(new Set<string>())
  const resolvedQuestions = useRef(new Set<string>())
  const requestSequence = useRef(0)
  const requestEvents = useRef<
    { sequence: number; workspaceId: string; type: string; data: any }[]
  >([])
  const requestSnapshots = useRef(0)
  current.current = { project, session }
  useEffect(() => {
    if (!project) return
    const projectId = project.id
    return watchProjectSessions([projectId], window.prime, (id, snapshot) => {
      if (current.current.project?.id !== id) return
      const sessionId = current.current.session?.id
      if (!sessionId) return
      setSessions(snapshot)
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
    if (!project || !session) return
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
      const results = await Promise.allSettled(tasks.map(([, task]) => task))
      if (current.current.project?.id !== project.id) return
      const values: Record<string, Json> = {}
      tasks.forEach(([name], index) => {
        values[name] =
          results[index].status === 'fulfilled'
            ? (results[index] as PromiseFulfilledResult<Json>).value
            : undefined
      })
      if (values.sessions) setSessions(values.sessions)
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
      let active: Session[] | undefined = values.sessions
      if (includeMessages) {
        const snapshotSequence = messageSequence.current
        messageSnapshots.current++
        try {
          const [msgs, latest] = await Promise.allSettled([
            window.prime.call(project.id, 'messagePage', { sessionId: session.id, limit: 60 }),
            window.prime.call(project.id, 'sessions')
          ])
          if (
            current.current.project?.id !== project.id ||
            current.current.session?.id !== session.id
          )
            return
          if (msgs.status === 'fulfilled') {
            let reconciled = msgs.value as MessagePage
            for (const entry of messageEvents.current)
              if (
                entry.sequence > snapshotSequence &&
                entry.workspaceId === project.id &&
                entry.message.session_id === session.id
              )
                reconciled = updateHistoryMessage(reconciled, entry.message, entry.action)
            history.accept(session.id, reconciled)
          } else history.fail(session.id, msgs.reason)
          active = latest.status === 'fulfilled' ? latest.value : undefined
        } finally {
          if (--messageSnapshots.current === 0) messageEvents.current = []
        }
      }
      if (current.current.project?.id !== project.id || current.current.session?.id !== session.id)
        return
      const selected = active?.find((item: Session) => item.id === session.id)
      if (selected) {
        setSession(selected)
        setBusy(Boolean(selected.is_busy))
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
        await refresh()
      }
      setStartupReady(true)
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
    const unsubscribe = window.prime.onEvent((event) => {
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
        setInfo((current: Json) => ({ ...current, terminalCount: event.payload.activeCount }))
        return
      }
      if (event.type === 'recovered') {
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
        if (event.payload.status === 'ready') void restore().catch(errorHandler)
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
        if (messageSnapshots.current > 0) {
          messageEvents.current.push({
            sequence: messageSequence.current,
            workspaceId: event.workspaceId,
            action: event.payload.action,
            message: payload
          })
          if (messageEvents.current.length > 2000) messageEvents.current.splice(0, 1000)
        }
        if (payload?.session_id === current.current.session?.id)
          history.live(payload, event.payload.action)
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
        void refresh()
      }
      if (event.type === 'agent_event') {
        if (payload?.type === 'error' && !/context canceled/i.test(payload.error || ''))
          setError(payload.error || translate(language.current, 'Ошибка агента'))
        void refresh()
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
      if (
        [
          'config_changed',
          'mcp_event',
          'lsp_event',
          'skills_event',
          'mcp_channel_message'
        ].includes(event.type)
      )
        void refresh()
    })
    return unsubscribe
  }, [refresh, refreshPending, errorHandler])
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
  async function openWorkspace(open: () => Promise<Project | null>) {
    setLoading(true)
    setError('')
    try {
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
      setProjects(await window.prime.projects())
      setPreferences(await window.prime.preferences())
      await refresh()
    } catch (error) {
      errorHandler(error)
    } finally {
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
    await refreshLight()
    void scheduleRefresh(false, true).catch(() => {})
  }
  async function closeProject(id: string) {
    try {
      await window.prime.closeProject(id)
      setPreferences(await window.prime.preferences())
      const remaining = projects.filter((item) => item.id !== id)
      setProjects(remaining)
      if (current.current.project?.id === id) {
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
    } catch (error) {
      errorHandler(error)
    }
  }
  async function selectSession(selected: Session) {
    const cached = history.select(selected.id)
    setSession(selected)
    current.current.session = selected
    setBusy(Boolean(selected.is_busy))
    setQueue([])
    await call('currentSession', { sessionId: selected.id }).catch(errorHandler)
    if (cached) await scheduleRefresh(true, false)
    else await refresh()
  }
  async function openSession(target: Project, selected: Session, known?: Session[]) {
    if (target.id === current.current.project?.id) return selectSession(selected)
    const cached = history.select(selected.id)
    setDraftReasoningMode(null)
    current.current = { project: target, session: selected }
    setProject(target)
    setSession(selected)
    if (known) setSessions(known)
    setBusy(Boolean(selected.is_busy))
    setQueue([])
    setPermissions([])
    setQuestions([])
    await window.prime
      .call(target.id, 'currentSession', { sessionId: selected.id })
      .catch(errorHandler)
    if (cached) await scheduleRefresh(true, false)
    else await refresh()
  }
  async function openChat() {
    try {
      const opened = await window.prime.openChat()
      setProjects(await window.prime.projects())
      await selectProject(opened)
      const existing: Session[] = await window.prime.call(opened.id, 'sessions')
      const saved = await window.prime.preferences()
      const selected =
        existing.find(
          (item) => item.id === (opened.currentSessionId || saved.activeSessions?.[opened.path])
        ) || [...existing].sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0))[0]
      if (selected) await selectSession(selected)
    } catch (error) {
      errorHandler(error)
    }
  }
  async function createSession() {
    try {
      const targetProject =
        current.current.project ||
        projects.find((item) => item.kind === 'chat') ||
        (await window.prime.openChat())
      if (targetProject.id !== current.current.project?.id) {
        setProjects(await window.prime.projects())
        await selectProject(targetProject)
      }
      const created = await window.prime.call(targetProject.id, 'createSession', {
        title: translate(language.current, 'Новый чат')
      })
      setSessions((items) => upsertSession(items, created))
      void savePreferences({
        reasoningBySession: { [created.id]: draftReasoningMode || reasoningFallback }
      }).catch(errorHandler)
      setDraftReasoningMode(null)
      await selectSession(created)
      return created as Session
    } catch (error) {
      errorHandler(error)
      return null
    }
  }
  async function send(prompt: string, attachmentIds: string[] = []) {
    if (!prompt.trim()) return false
    const targetProject = current.current.project
    if (!targetProject) return false
    const targetSession = current.current.session
    const usedModel = config.models?.large
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
      await refresh()
      return true
    } catch (error) {
      errorHandler(error)
      await refresh()
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
    sessionId: session?.id,
    preferences,
    savePreferences,
    onError: errorHandler
  })
  const reasoningFallback = preferences.lastReasoningMode || 'off'
  const reasoningMode = session
    ? preferences.reasoningBySession?.[session.id] || reasoningFallback
    : draftReasoningMode || reasoningFallback
  async function setReasoningMode(mode: ChatReasoningMode) {
    const selected = current.current.session
    if (!selected) setDraftReasoningMode(mode)
    await savePreferences({
      lastReasoningMode: mode,
      ...(selected ? { reasoningBySession: { [selected.id]: mode } } : {})
    })
  }
  async function deleteSession(id: string) {
    await call('deleteSession', { sessionId: id })
    history.delete(id)
    setSessions((items) => items.filter((item) => item.id !== id))
    if (current.current.session?.id === id) {
      current.current.session = null
      setSession(null)
      history.reset()
      setBusy(false)
      setQueue([])
    }
    await refresh()
  }
  return {
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
    messages,
    history,
    loadHistory: (direction: 'before' | 'after' | 'latest') => {
      const { project, session } = current.current
      return project && session
        ? history.load(project.id, session.id, direction)
        : Promise.resolve()
    },
    searchMessages: useCallback(
      (query: string) => call('messageSearch', { sessionId: current.current.session?.id, query }),
      [call]
    ),
    revealMessage: useCallback(
      (id: string) => {
        const { project, session } = current.current
        return project && session ? history.reveal(project.id, session.id, id) : Promise.resolve()
      },
      [history.reveal]
    ),
    messagesReady: !session || messagesSessionId === session.id,
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
    openProject,
    openProjectSession,
    openProjectNewChat,
    openRemote,
    selectProject,
    closeProject,
    selectSession,
    openSession,
    createSession,
    deleteSession,
    send
  }
}
export type Workspace = ReturnType<typeof useWorkspace>
