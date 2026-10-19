import { useCallback, useEffect, useRef, useState } from 'react'
import type {
  Json,
  Message,
  Operation,
  Preferences,
  QueuedPrompt,
  Project,
  Session
} from '../../../shared/contracts'
import { applyMessage } from '../../../shared/messages'
import { translate } from '../i18n'
import { useAgentContext } from '../features/context/useAgentContext'
import { buildAgentPrompt } from '../../../shared/agentContext'

export function useWorkspace() {
  const [info, setInfo] = useState<Json>({ runtime: 'starting' })
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
  const [messages, setMessages] = useState<Message[]>([])
  const [permissions, setPermissions] = useState<Json[]>([])
  const [questions, setQuestions] = useState<Json[]>([])
  const [config, setConfig] = useState<Json>({})
  const [agent, setAgent] = useState<Json>({})
  const [queue, setQueue] = useState<QueuedPrompt[]>([])
  const [resources, setResources] = useState<Json>({ mcp: {}, lsp: {}, skills: [] })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [pendingShell, setPendingShell] = useState<{ projectId: string; sessionId: string } | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const current = useRef({ project, session })
  const messageSequence = useRef(0)
  const messageEvents = useRef<
    { sequence: number; workspaceId: string; action: string; message: Message }[]
  >([])
  const resolvedPermissions = useRef(new Set<string>())
  const resolvedQuestions = useRef(new Set<string>())
  const requestSequence = useRef(0)
  const requestEvents = useRef<
    { sequence: number; workspaceId: string; type: string; data: any }[]
  >([])
  current.current = { project, session }
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
    try {
      await request
    } finally {
      pendingFetches.current.delete(project.id)
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
  const refresh = useCallback(async () => {
    const { project, session } = current.current
    if (!project) return
    const results = await Promise.allSettled([
      window.prime.call(project.id, 'sessions'),
      window.prime.call(project.id, 'config'),
      window.prime.call(project.id, 'agent'),
      refreshPending(),
      window.prime.call(project.id, 'mcp'),
      window.prime.call(project.id, 'lsp'),
      window.prime.call(project.id, 'skills'),
      window.prime.call(project.id, 'channelMessages'),
      refreshQueue()
    ])
    if (current.current.project?.id !== project.id) return
    const get = (index: number) =>
      results[index].status === 'fulfilled'
        ? (results[index] as PromiseFulfilledResult<Json>).value
        : undefined
    if (get(0)) setSessions(get(0))
    if (get(1)) setConfig(get(1))
    if (get(2)) setAgent(get(2))
    setResources({
      mcp: get(4) || {},
      lsp: get(5) || {},
      skills: get(6) || [],
      channels: get(7) || []
    })
    if (session) {
      const snapshotSequence = messageSequence.current
      const [msgs, active] = await Promise.allSettled([
        window.prime.call(project.id, 'messages', { sessionId: session.id }),
        window.prime.call(project.id, 'sessions')
      ])
      if (current.current.session?.id !== session.id) return
      if (msgs.status === 'fulfilled') {
        let reconciled: Message[] = msgs.value || []
        for (const entry of messageEvents.current)
          if (
            entry.sequence > snapshotSequence &&
            entry.workspaceId === project.id &&
            entry.message.session_id === session.id
          )
            reconciled =
              entry.action === 'deleted'
                ? reconciled.filter((item) => item.id !== entry.message.id)
                : applyMessage(reconciled, entry.message)
        setMessages(reconciled)
      }
      if (active.status === 'fulfilled') {
        const selected = active.value?.find((item: Session) => item.id === session.id)
        if (selected) {
          setSession(selected)
          setBusy(Boolean(selected.is_busy))
        }
      }
    }
  }, [refreshPending, refreshQueue])
  useEffect(() => {
    async function restore() {
      const saved = await window.prime.preferences()
      let opened = await window.prime.projects()
      const lastProject = saved.activeProject
        ? saved.recent.includes(saved.activeProject)
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
        const status = await window.prime.call(restored.id, 'agent')
        if (!status?.is_ready)
          await window.prime
            .call(restored.id, 'init')
            .catch(restored.kind === 'chat' ? () => {} : errorHandler)
        await refresh()
      }
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
        ].includes(event.type)
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
        messageEvents.current.push({
          sequence: ++messageSequence.current,
          workspaceId: event.workspaceId,
          action: event.payload.action,
          message: payload
        })
        if (messageEvents.current.length > 10000) messageEvents.current.splice(0, 5000)
        if (payload?.session_id === current.current.session?.id)
          setMessages((items) =>
            event.payload.action === 'deleted'
              ? items.filter((item) => item.id !== payload.id)
              : applyMessage(items, payload)
          )
      }
      if (event.type === 'session') {
        setSessions((items) => {
          const remaining = items.filter((item) => item.id !== payload.id)
          return event.payload.action === 'deleted' ? remaining : [payload, ...remaining]
        })
        if (payload?.id === current.current.session?.id)
          setSession((previous: any) => ({ ...previous, ...payload }))
      }
      if (event.type === 'run_complete') {
        if (payload.session_id === current.current.session?.id) {
          setBusy(false)
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
        resolvedPermissions.current.add(`${event.workspaceId}:${payload.tool_call_id}`)
        setPermissions((items) =>
          items.filter((item) => item.tool_call_id !== payload.tool_call_id)
        )
      }
      if (event.type === 'question_batch_request')
        setQuestions((items) => [...items.filter((item) => item.id !== payload.id), payload])
      if (event.type === 'question_batch_notification') {
        resolvedQuestions.current.add(`${event.workspaceId}:${payload.batch_id}`)
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
    const timer = setInterval(() => void refresh().catch(() => {}), 10000)
    return () => clearInterval(timer)
  }, [project?.id, refresh])
  async function openProject(path?: string) {
    setLoading(true)
    setError('')
    try {
      const opened = await window.prime.openProject(path)
      if (!opened) return
      current.current = { project: opened, session: null }
      setProject(opened)
      setSession(null)
      setMessages([])
      setPermissions([])
      setQuestions([])
      setBusy(false)
      setQueue([])
      setProjects(await window.prime.projects())
      setPreferences(await window.prime.preferences())
      await refresh()
      try {
        await window.prime.call(opened.id, 'init')
      } catch (error) {
        errorHandler(error)
      }
      await refresh()
    } catch (error) {
      errorHandler(error)
    } finally {
      setLoading(false)
    }
  }
  async function selectProject(selected: Project) {
    current.current = { project: selected, session: null }
    setProject(selected)
    setSession(null)
    setMessages([])
    setBusy(false)
    setQueue([])
    await refresh()
  }
  async function closeProject(id: string) {
    try {
      await window.prime.closeProject(id)
      const remaining = projects.filter((item) => item.id !== id)
      setProjects(remaining)
      if (current.current.project?.id === id) {
        current.current = { project: null, session: null }
        setProject(null)
        setSession(null)
        setMessages([])
        setSessions([])
        setBusy(false)
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
    setSession(selected)
    current.current.session = selected
    setMessages([])
    setBusy(Boolean(selected.is_busy))
    setQueue([])
    await call('currentSession', { sessionId: selected.id }).catch(errorHandler)
    await refresh()
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
      const created = await call('createSession', {
        title: translate(language.current, 'Новый чат')
      })
      setSessions((items) => [created, ...items])
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
    try {
      const preparedPrompt = await buildAgentPrompt(prompt, agentContext.context, (path) =>
        window.prime.readFile(targetProject.id, path)
      )
      let selected = targetSession
      if (!selected) {
        selected = await window.prime.call(targetProject.id, 'createSession', {
          title: prompt.slice(0, 60)
        })
      }
      if (!selected) return false
      const runId = crypto.randomUUID()
      if (current.current.session?.id === selected.id) setBusy(true)
      if (prompt.startsWith('!'))
        setPendingShell({ projectId: targetProject.id, sessionId: selected.id })
      if (prompt.startsWith('!'))
        await window.prime.call(targetProject.id, 'shell', {
          sessionId: selected.id,
          command: prompt.slice(1).trim()
        })
      else
        await window.prime.call(targetProject.id, 'send', {
          sessionId: selected.id,
          runId,
          prompt: preparedPrompt,
          attachmentIds
        })
      if (current.current.project?.id === targetProject.id && !current.current.session) {
        setSessions((items) => [selected!, ...items])
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
  async function deleteSession(id: string) {
    await call('deleteSession', { sessionId: id })
    if (current.current.session?.id === id) {
      current.current.session = null
      setSession(null)
      setMessages([])
      setBusy(false)
      setQueue([])
    }
    await refresh()
  }
  return {
    agentContext: agentContext.context,
    updateAgentContext: agentContext.updateContext,
    info,
    preferences,
    savePreferences,
    projects,
    project,
    sessions,
    session,
    messages,
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
      busy ||
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
    selectProject,
    closeProject,
    selectSession,
    createSession,
    deleteSession,
    send
  }
}
export type Workspace = ReturnType<typeof useWorkspace>
