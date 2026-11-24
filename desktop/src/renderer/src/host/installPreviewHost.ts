import type {
  AccountAction,
  AccountState,
  DesktopApi,
  DesktopEvent,
  Preferences,
  PrimeAILimits,
  Project,
  Session
} from '../../../shared/contracts'
import { notifyWarning } from '../features/notifications/notifications'
import {
  createClosedProjectSessions,
  createPreviewAutomations,
  createPreviewBlackboard,
  createPreviewBrowserLogs,
  createPreviewBrowserState,
  createPreviewConfig,
  createPreviewCustomAgentModels,
  createPreviewCustomAgentRuns,
  createPreviewCustomAgents,
  createPreviewCustomWorkflows,
  createPreviewGlobalMemory,
  createPreviewDesignerProject,
  createPreviewFileContents,
  createPreviewFiles,
  createPreviewJobs,
  createPreviewLaunch,
  createPreviewMessages,
  createPreviewPreferences,
  createPreviewProjects,
  createPreviewProviders,
  createPreviewRemote,
  createPreviewSessions,
  createPreviewTeams,
  createPreviewTerminal,
  createPreviewWorkers,
  previewDemoVideoDataUrl,
  previewDesignPngBase64
} from './previewHostData'
import type {
  BlackboardEntry,
  CustomAgent,
  CustomAgentRun,
  CustomAgentsRequest,
  CustomWorkflow,
  GlobalMemoryDoc,
  SwarmResult,
  SwarmWorkerResult
} from '../../../shared/customAgents'
import {
  fileNameFromPath,
  isDirectMediaUrl,
  isVideoPath,
  videoMimeFromPath
} from '../features/chat/video/videoMedia'

export function installPreviewHost() {
  const listeners = new Set<(event: DesktopEvent) => void>()
  const emit = (event: DesktopEvent) => {
    for (const listener of [...listeners]) listener(event)
  }

  let preferences: Preferences = createPreviewPreferences()
  let projects: Project[] = createPreviewProjects()
  const sessionsByProject = createPreviewSessions()
  const closedSessions = createClosedProjectSessions()
  const messagesBySession = createPreviewMessages()
  const workspaceModes = new Map<string, string>([['ws-designer', 'designer']])
  const previewAttachments = new Map<
    string,
    { id: string; name: string; size: number; mimeType: string; base64: string; preview: string }
  >()
  const nextAvailablePreviewName = (baseName: string) => {
    const taken = new Set(
      [
        ...projects.filter((item) => item.kind === 'project').map((item) => item.path),
        ...preferences.recent
      ]
        .map((item) => item.split(/[\\/]/).pop()?.toLowerCase() || '')
        .filter(Boolean)
    )
    const trimmed = baseName.trim() || 'Новый проект'
    if (!taken.has(trimmed.toLowerCase())) return trimmed
    let index = 1
    while (taken.has(`${trimmed} ${index}`.toLowerCase())) index++
    return `${trimmed} ${index}`
  }
  const workers = createPreviewWorkers()
  const workerMessages: Record<string, any[]> = {
    'worker-sess-1': [
      {
        id: 'wmsg-1',
        session_id: 'worker-sess-1',
        role: 'assistant',
        created_at: Math.floor(Date.now() / 1000) - 180,
        parts: [
          {
            type: 'text',
            data: {
              text: 'Проверяю отступы `.workspace-topbar` на ширинах 700–960px и навигацию настроек на 761–860px.'
            }
          }
        ]
      }
    ],
    'worker-sess-2': [
      {
        id: 'wmsg-2',
        session_id: 'worker-sess-2',
        role: 'assistant',
        created_at: Math.floor(Date.now() / 1000) - 240,
        parts: [
          {
            type: 'text',
            data: {
              text: 'Контрастность активной вкладки и теней в светлой и тёмной темах проверена.'
            }
          }
        ]
      }
    ]
  }

  let permissions: any[] = [
    {
      id: 'perm-preview-1',
      session_id: 'sess-1',
      tool_call_id: 'tc-perm-1',
      tool_name: 'bash',
      description: 'Запустить unit-тесты Vite для проверки регрессий',
      path: 'desktop/tests/unit',
      params: { command: 'npm run test:unit' }
    }
  ]

  const filesTree = createPreviewFiles()
  const fileContents = createPreviewFileContents()
  const config = createPreviewConfig()
  const providers = createPreviewProviders()
  const terminals = new Map<string, ReturnType<typeof createPreviewTerminal>>([
    ['term-1', createPreviewTerminal('ws-prime')]
  ])
  const terminalOutput = new Map<string, string>([
    [
      'term-1',
      'PS C:\\Users\\demo\\Projects\\prime-desktop> npm run test:unit\r\n\x1b[32m✓\x1b[0m 26 test files passed\r\nPS C:\\Users\\demo\\Projects\\prime-desktop> '
    ]
  ])
  let jobs = createPreviewJobs()
  let automations = createPreviewAutomations()
  let launchState = createPreviewLaunch()
  let teamsState = createPreviewTeams()
  let remoteState = createPreviewRemote()
  let browserState = createPreviewBrowserState()
  let browserLogs = createPreviewBrowserLogs()
  let customWorkflows: CustomWorkflow[] = createPreviewCustomWorkflows()
  let customAgents: CustomAgent[] = createPreviewCustomAgents()
  let customAgentRuns: CustomAgentRun[] = createPreviewCustomAgentRuns()
  let customBlackboard: BlackboardEntry[] = createPreviewBlackboard()
  let globalMemory: GlobalMemoryDoc = createPreviewGlobalMemory()
  const customModels = createPreviewCustomAgentModels()
  const customRunTimers = new Map<string, ReturnType<typeof setTimeout>[]>()

  const search = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null
  const authMode = search?.get('auth') || ''
  const limitsMode = search?.get('limits') || ''
  let storedAuth = ''
  try {
    storedAuth = typeof localStorage !== 'undefined' ? localStorage.getItem('prime:preview-auth') || '' : ''
  } catch {}
  if (authMode === 'signed-in') {
    try {
      localStorage.removeItem('prime:preview-auth')
    } catch {}
  }
  const initialSignedOut = authMode !== 'signed-in' && (authMode === 'signed-out' || storedAuth === 'signed-out')
  const previewLoginUrl = 'https://vpn.sphereprime.ru/auth/google?app_hash=desktop_preview_demo_hash'
  let previewNetworkError = authMode === 'network-error'
  let previewLoginTimer: ReturnType<typeof setTimeout> | null = null
  let accountState: AccountState =
    authMode === 'waiting'
      ? {
          signedIn: false,
          waiting: true,
          checking: false,
          email: '',
          loginUrl: previewLoginUrl,
          error: '',
          networkError: false,
          hasStoredSession: false
        }
      : previewNetworkError
        ? {
            signedIn: false,
            waiting: false,
            checking: false,
            email: '',
            loginUrl: '',
            error: 'Не удалось проверить сессию из-за ошибки сети. Повторите проверку или войдите заново.',
            networkError: true,
            hasStoredSession: true
          }
        : initialSignedOut
          ? {
              signedIn: false,
              waiting: false,
              checking: false,
              email: '',
              loginUrl: '',
              error: '',
              networkError: false,
              hasStoredSession: false
            }
          : {
              signedIn: true,
              waiting: false,
              checking: false,
              email: 'preview@sphereprime.ru',
              loginUrl: '',
              error: '',
              networkError: false,
              hasStoredSession: true
            }

  const clearPreviewLoginTimer = () => {
    if (previewLoginTimer) {
      clearTimeout(previewLoginTimer)
      previewLoginTimer = null
    }
  }

  const buildPreviewLimits = (): PrimeAILimits => {
    if (!accountState.signedIn) {
      return { available: false, cacheWeightPercent: 0 }
    }
    const now = Date.now()
    const fiveHourResetInSeconds = 3 * 3600 + 12 * 60
    const weeklyResetInSeconds = 4 * 86400 + 6 * 3600
    const preset =
      limitsMode === 'exceeded'
        ? {
            fiveHour: {
              usedTokens: 500_000,
              limitTokens: 500_000,
              usedRequests: 0,
              limitRequests: 0,
              percent: 100
            },
            weekly: {
              usedTokens: 4_750_000,
              limitTokens: 5_000_000,
              usedRequests: 0,
              limitRequests: 0,
              percent: 95
            }
          }
        : limitsMode === 'high'
          ? {
              fiveHour: {
                usedTokens: 410_000,
                limitTokens: 500_000,
                usedRequests: 0,
                limitRequests: 0,
                percent: 82
              },
              weekly: {
                usedTokens: 3_200_000,
                limitTokens: 5_000_000,
                usedRequests: 0,
                limitRequests: 0,
                percent: 64
              }
            }
          : {
              fiveHour: {
                usedTokens: 142_500,
                limitTokens: 500_000,
                usedRequests: 0,
                limitRequests: 0,
                percent: 28.5
              },
              weekly: {
                usedTokens: 710_000,
                limitTokens: 5_000_000,
                usedRequests: 0,
                limitRequests: 0,
                percent: 14.2
              }
            }
    return {
      available: true,
      planName: 'Pro',
      cacheWeightPercent: 0,
      fiveHour: {
        ...preset.fiveHour,
        resetAt: new Date(now + fiveHourResetInSeconds * 1000).toISOString(),
        resetInSeconds: fiveHourResetInSeconds
      },
      weekly: {
        ...preset.weekly,
        resetAt: new Date(now + weeklyResetInSeconds * 1000).toISOString(),
        resetInSeconds: weeklyResetInSeconds
      },
      updatedAt: new Date(now).toISOString()
    }
  }

  function account(action: 'status' | 'restore' | 'login' | 'logout'): Promise<AccountState>
  function account(action: 'limits' | 'refreshLimits'): Promise<PrimeAILimits>
  function account(action: AccountAction): Promise<AccountState | PrimeAILimits>
  async function account(action: AccountAction): Promise<AccountState | PrimeAILimits> {
    if (action === 'limits' || action === 'refreshLimits') return buildPreviewLimits()
    if (action === 'status') return { ...accountState }
    if (action === 'restore') {
      if (previewNetworkError) {
        accountState = {
          ...accountState,
          signedIn: false,
          waiting: false,
          checking: false,
          networkError: true,
          hasStoredSession: true,
          error: 'Не удалось проверить сессию из-за ошибки сети. Повторите проверку или войдите заново.'
        }
        return { ...accountState }
      }
      accountState = { ...accountState, checking: false }
      return { ...accountState }
    }
    if (action === 'login') {
      clearPreviewLoginTimer()
      previewNetworkError = false
      accountState = {
        signedIn: false,
        waiting: true,
        checking: false,
        email: '',
        loginUrl: previewLoginUrl,
        error: '',
        networkError: false,
        hasStoredSession: false
      }
      if (authMode !== 'waiting' && authMode !== 'manual') {
        previewLoginTimer = setTimeout(() => {
          previewLoginTimer = null
          try {
            localStorage.removeItem('prime:preview-auth')
          } catch {}
          accountState = {
            signedIn: true,
            waiting: false,
            checking: false,
            email: 'preview@sphereprime.ru',
            loginUrl: '',
            error: '',
            networkError: false,
            hasStoredSession: true
          }
        }, 2500)
      }
      return { ...accountState }
    }
    clearPreviewLoginTimer()
    previewNetworkError = false
    try {
      localStorage.setItem('prime:preview-auth', 'signed-out')
    } catch {}
    accountState = {
      signedIn: false,
      waiting: false,
      checking: false,
      email: '',
      loginUrl: '',
      error: '',
      networkError: false,
      hasStoredSession: false
    }
    return { ...accountState }
  }

  const api: DesktopApi = {
    async automation(_workspaceId, request) {
      if (request.action === 'save') {
        const now = Date.now()
        const id = request.input.id || `auto-${now}`
        const next = {
          ...request.input,
          id,
          root: request.root,
          createdAt: now,
          updatedAt: now
        }
        automations = {
          ...automations,
          rules: [...automations.rules.filter((item) => item.id !== id), next]
        }
      } else if (request.action === 'remove') {
        automations = {
          ...automations,
          rules: automations.rules.filter((item) => item.id !== request.id)
        }
      }
      return automations
    },
    async launch(_workspaceId, request) {
      if (request.action === 'save') {
        const id = request.input.id || `launch-${Date.now()}`
        launchState = {
          ...launchState,
          configs: [...launchState.configs.filter((item) => item.id !== id), { ...request.input, id }]
        }
      } else if (request.action === 'remove') {
        launchState = {
          ...launchState,
          configs: launchState.configs.filter((item) => item.id !== request.id)
        }
      }
      return launchState
    },
    async browserLogs(clear) {
      if (clear) browserLogs = []
      return browserLogs
    },
    async speech(request) {
      if (request.action === 'voices') return [{ id: 'F1', name: 'Женский 1', language: 'ru' }]
      if (request.action === 'status') return { available: false, ready: false }
      return {}
    },
    async github(_workspaceId, request) {
      const commit = {
        sha: 'c22d0c42a1b2c3d4e5f60718293a4b5c6d7e8f90',
        parents: ['7faf040c11223344556677889900aabbccddeeff'],
        title: 'desktop: refine dock and inspector geometry',
        author: 'SpherePrime',
        date: new Date().toISOString(),
        refs: 'HEAD -> main'
      }
      if (request.action === 'history') {
        return {
          commits: [commit],
          currentHead: commit.sha,
          branch: 'main',
          clean: false
        }
      }
      if (request.action === 'showCommit') {
        return {
          commit,
          currentHead: commit.sha,
          diff: 'diff --git a/src/app/App.tsx b/src/app/App.tsx\n--- a/src/app/App.tsx\n+++ b/src/app/App.tsx\n@@ -1,3 +1,3 @@\n-const workbench = false\n+const workbench = true\n',
          truncated: false
        }
      }
      if (request.action === 'status') return { available: true, repository: 'SpherePrime/CLI', branch: 'main' }
      if (request.action === 'issues' || request.action === 'pulls') return []
      if (request.action === 'branches') return ['main']
      return {}
    },
    async windows(_workspaceId, request) {
      if (request.action === 'list') {
        return [
          {
            hwnd: '0x00120A',
            title: 'Prime Desktop — Preview',
            pid: 1024,
            identity: 'prime-desktop',
            bounds: { x: 0, y: 0, width: 1440, height: 900 }
          }
        ]
      }
      if (request.action === 'state') return { log: [] }
      return { log: [] }
    },
    async appTesting(_workspaceId, request) {
      if (request.action === 'targets') {
        return {
          windows: [
            {
              hwnd: '0x00120A',
              title: 'Prime Desktop — Preview',
              pid: 1024,
              identity: 'prime-desktop',
              bounds: { x: 0, y: 0, width: 1440, height: 900 }
            }
          ]
        }
      }
      return { target: null, log: [], report: { checks: [] } }
    },
    async appPick() {
      throw new Error('Cancelled')
    },
    async appPickCancel() {},
    async jobs(request) {
      if (request.action === 'stop') {
        jobs = jobs.map((job) =>
          job.id === request.id ? { ...job, status: 'cancelled', stoppable: false } : job
        )
      }
      return jobs
    },
    async browserPick() {
      return null
    },
    async browserPickCancel() {},
    async team(_workspaceId, request) {
      if (request.action === 'remove') {
        teamsState = {
          ...teamsState,
          teams: teamsState.teams.filter((team) => team.id !== request.teamId)
        }
      }
      return teamsState
    },
    async terminal(workspaceId, request) {
      if (request.action === 'list') {
        return [...terminals.values()].filter((term) => term.workspaceId === workspaceId)
      }
      if (request.action === 'create') {
        const id = `term-${terminals.size + 1}`
        const created = { ...createPreviewTerminal(workspaceId), id, title: `pwsh ${terminals.size + 1}` }
        terminals.set(id, created)
        terminalOutput.set(id, 'PS> ')
        emit({ workspaceId, type: 'terminal', payload: { action: 'state', terminal: created } })
        return created
      }
      if (request.action === 'read') {
        const term = terminals.get(request.id) ?? createPreviewTerminal(workspaceId)
        return {
          terminal: term,
          output: terminalOutput.get(request.id) || '',
          sequence: term.sequence,
          truncated: false
        }
      }
      if (request.action === 'write') {
        const prev = terminalOutput.get(request.id) || ''
        const next = prev + request.data
        terminalOutput.set(request.id, next)
        const term = terminals.get(request.id)
        if (term) term.sequence++
        emit({
          workspaceId,
          type: 'terminal',
          payload: { action: 'data', id: request.id, data: request.data, sequence: term?.sequence || 1 }
        })
        return
      }
      if (request.action === 'close') {
        const term = terminals.get(request.id)
        terminals.delete(request.id)
        if (term) {
          emit({
            workspaceId,
            type: 'terminal',
            payload: { action: 'exit', id: request.id, exitCode: 0, sequence: term.sequence + 1 }
          })
        }
        return
      }
      return
    },
    async changes() {
      return [
        { path: 'src/app/App.tsx', status: 'modified' },
        { path: 'src/styles/workbench-dock.css', status: 'modified' }
      ]
    },
    async info() {
      return {
        runtime: 'ready',
        host: 'preview',
        version: '0.4.105',
        runtimeVersion: { version: '0.4.105-preview' },
        terminalCount: terminals.size,
        automationCounts: { activeCount: 0, enabledCount: automations.rules.length }
      }
    },
    async projects() {
      return projects
    },
    async projectSessions(paths) {
      const result: Record<string, Session[]> = {}
      for (const path of paths) result[path] = closedSessions[path] || []
      return result
    },
    async openChat() {
      return projects.find((item) => item.kind === 'chat') || projects[0]
    },
    async openDesigner() {
      const existing = projects.find((item) => item.kind === 'designer')
      if (existing) return existing
      const created = createPreviewDesignerProject()
      projects = [...projects, created]
      sessionsByProject[created.id] = sessionsByProject[created.id] || []
      workspaceModes.set(created.id, 'designer')
      return created
    },
    async projectCreationBase() {
      return 'C:\\Users\\demo\\Projects'
    },
    async projectCreationNextName(defaultName) {
      return nextAvailablePreviewName(defaultName)
    },
    async createProject(name) {
      const resolvedName = nextAvailablePreviewName(name)
      const path = `C:\\Users\\demo\\Projects\\${resolvedName}`
      const created: Project = {
        id: `ws-${Date.now()}`,
        kind: 'project',
        path,
        clientId: 'preview-client',
        connection: 'ready'
      }
      projects = [...projects, created]
      sessionsByProject[created.id] = []
      preferences = {
        ...preferences,
        recent: [path, ...preferences.recent.filter((item) => item !== path)]
      }
      return created
    },
    async openProject(path) {
      if (!path) return null
      const existing = projects.find((item) => item.path === path)
      if (existing) return existing
      const opened: Project = {
        id: `ws-${Date.now()}`,
        kind: 'project',
        path,
        clientId: 'preview-client',
        connection: 'ready'
      }
      projects = [...projects, opened]
      sessionsByProject[opened.id] = closedSessions[path] || []
      return opened
    },
    async cancelProjectWait() {},
    async openRemote(profileId) {
      const existing = projects.find((item) => item.remote?.profileId === profileId)
      return existing || projects[0]
    },
    async remote(request) {
      if (request.action === 'remove') {
        remoteState = {
          ...remoteState,
          profiles: remoteState.profiles.filter((item) => item.id !== request.id)
        }
      }
      return remoteState
    },
    async remoteKey() {
      return undefined
    },
    async sudoPassword() {
      return { hasPassword: false, storageAvailable: true }
    },
    async deleteProject(path) {
      projects = projects.filter((item) => item.path !== path)
      preferences = { ...preferences, recent: preferences.recent.filter((item) => item !== path) }
      return { warnings: [] }
    },
    async call(workspaceId, operation, args) {
      if (operation === 'sessions') return sessionsByProject[workspaceId] || []
      if (operation === 'createSession') {
        const created: Session = {
          id: `sess-${Date.now()}`,
          title: args?.title || 'Новый чат',
          created_at: Math.floor(Date.now() / 1000),
          updated_at: Math.floor(Date.now() / 1000)
        }
        sessionsByProject[workspaceId] = [created, ...(sessionsByProject[workspaceId] || [])]
        messagesBySession[created.id] = []
        return created
      }
      if (operation === 'renameSession') {
        sessionsByProject[workspaceId] = (sessionsByProject[workspaceId] || []).map((item) =>
          item.id === args?.sessionId ? { ...item, title: args.session?.title || item.title } : item
        )
        return {}
      }
      if (operation === 'deleteSession') {
        sessionsByProject[workspaceId] = (sessionsByProject[workspaceId] || []).filter(
          (item) => item.id !== args?.sessionId
        )
        return {}
      }
      if (operation === 'messagePage') {
        const list = messagesBySession[args?.sessionId] || []
        return {
          messages: list,
          has_more: false,
          has_before: false,
          has_after: false,
          timing: {}
        }
      }
      if (operation === 'messageSearch') {
        const query = String(args?.query || '').toLowerCase()
        const list = messagesBySession[args?.sessionId] || []
        return list.filter((msg) => JSON.stringify(msg.parts).toLowerCase().includes(query))
      }
      if (operation === 'history') {
        return [{ path: 'src/app/App.tsx' }, { path: 'src/styles/workbench-dock.css' }]
      }
      if (operation === 'pending') {
        return { permissions, questions: [] }
      }
      if (operation === 'permission') {
        permissions = permissions.filter((item) => item.id !== args?.permission?.id)
        return { ok: true }
      }
      if (operation === 'mode') {
        const ws = projects.find((item) => item.id === workspaceId)
        const requested = String(args?.mode || 'general')
        if (ws?.kind === 'designer') {
          if (requested !== 'designer') throw new Error('Designer workspace requires designer mode')
        } else if (requested === 'designer') {
          throw new Error('Designer mode is only available in designer workspace')
        }
        workspaceModes.set(workspaceId, requested)
        return { mode: requested }
      }
      if (operation === 'send') {
        const ws = projects.find((item) => item.id === workspaceId)
        const sessionId = args?.sessionId
        const attachmentIds = Array.isArray(args?.attachmentIds) ? (args.attachmentIds as string[]) : []
        const attachmentParts = attachmentIds
          .map((id) => previewAttachments.get(id))
          .filter((item): item is NonNullable<typeof item> => Boolean(item))
          .map((item) => ({
            type: 'binary',
            data: {
              mime_type: item.mimeType,
              data: item.base64,
              path: item.name
            }
          }))
        const userMsg = {
          id: `msg-u-${Date.now()}`,
          session_id: sessionId,
          role: 'user',
          created_at: Math.floor(Date.now() / 1000),
          parts: [{ type: 'text', data: { text: args?.prompt || '' } }, ...attachmentParts]
        }
        messagesBySession[sessionId] = [...(messagesBySession[sessionId] || []), userMsg]
        emit({ workspaceId, type: 'message', payload: { action: 'created', data: userMsg } })
        setTimeout(() => {
          const isDesignerWs = ws?.kind === 'designer'
          const currentMode =
            workspaceModes.get(workspaceId) ||
            (isDesignerWs ? 'designer' : preferences.agentMode || 'general')
          const sessionOrchFlag = sessionId ? config.session_orchestrator?.[sessionId] : undefined
          const orchEnabled =
            currentMode === 'general' &&
            (sessionOrchFlag !== undefined
              ? Boolean(sessionOrchFlag)
              : Boolean(config.orchestrator?.enabled))
          if (orchEnabled) {
            const plannerModel =
              config.orchestrator?.planner?.model ||
              config.session_models?.[sessionId]?.model ||
              config.models?.large?.model ||
              'gpt-5.4'
            const executorModel =
              config.orchestrator?.executor?.model ||
              config.models?.small?.model ||
              config.models?.large?.model ||
              'claude-sonnet-4'
            const nowSec = Math.floor(Date.now() / 1000)
            const planMsg = {
              id: `msg-a-plan-${Date.now()}`,
              session_id: sessionId,
              role: 'assistant',
              model: plannerModel,
              orchestrator_phase: 'plan' as const,
              created_at: nowSec,
              parts: [
                {
                  type: 'text',
                  data: {
                    text: `План действий по запросу «${String(args?.prompt || '').slice(0, 80)}» подготовлен.`
                  }
                }
              ]
            }
            const execMsg = {
              id: `msg-a-exec-${Date.now() + 1}`,
              session_id: sessionId,
              role: 'assistant',
              model: executorModel,
              orchestrator_phase: 'execute' as const,
              created_at: nowSec + 1,
              parts: [
                {
                  type: 'text',
                  data: {
                    text: 'Изменения выполнены согласно плану.'
                  }
                },
                ...(config.orchestrator?.final_review === false
                  ? [{ type: 'finish', data: { reason: 'stop' } }]
                  : [])
              ]
            }
            const orchMessages: any[] = [planMsg, execMsg]
            if (config.orchestrator?.final_review !== false) {
              orchMessages.push({
                id: `msg-a-rev-${Date.now() + 2}`,
                session_id: sessionId,
                role: 'assistant',
                model: plannerModel,
                orchestrator_phase: 'review' as const,
                created_at: nowSec + 2,
                parts: [
                  {
                    type: 'text',
                    data: {
                      text: 'Финальная проверка пройдена: всё соответствует плану.'
                    }
                  },
                  { type: 'finish', data: { reason: 'stop' } }
                ]
              })
            }
            messagesBySession[sessionId] = [...(messagesBySession[sessionId] || []), ...orchMessages]
            for (const msg of orchMessages) {
              emit({ workspaceId, type: 'message', payload: { action: 'created', data: msg } })
            }
            emit({ workspaceId, type: 'run_complete', payload: { data: { session_id: sessionId } } })
            return
          }
          const replyParts: any[] = isDesignerWs
            ? [
                {
                  type: 'tool_call',
                  data: {
                    id: `call-gen-${Date.now()}`,
                    name: 'generate_image',
                    input: { prompt: String(args?.prompt || '').slice(0, 120) },
                    result: {
                      content: 'Generated concept.png',
                      mime_type: 'image/png',
                      data: previewDesignPngBase64,
                      images: [
                        {
                          data: previewDesignPngBase64,
                          mime_type: 'image/png',
                          path: 'concept.png'
                        }
                      ],
                      is_error: false
                    }
                  }
                },
                {
                  type: 'text',
                  data: {
                    text: `### Дизайн-концепт готов\n\nПроработана визуальная иерархия, акценты и адаптивная сетка по запросу:\n\n> ${String(args?.prompt || '').slice(0, 140)}`
                  }
                },
                { type: 'finish', data: { reason: 'stop' } }
              ]
            : [
                {
                  type: 'text',
                  data: {
                    text: attachmentParts.length
                      ? `Дизайн и прикреплённые макеты (${attachmentParts.length}) приняты в работу в проекте. Начинаю реализацию.`
                      : `Готово! Получен запрос в режиме предпросмотра:\n\n> ${String(args?.prompt || '').slice(0, 140)}`
                  }
                },
                { type: 'finish', data: { reason: 'stop' } }
              ]
          const reply = {
            id: `msg-a-${Date.now()}`,
            session_id: sessionId,
            role: 'assistant',
            created_at: Math.floor(Date.now() / 1000),
            parts: replyParts
          }
          messagesBySession[sessionId] = [...(messagesBySession[sessionId] || []), reply]
          emit({ workspaceId, type: 'message', payload: { action: 'created', data: reply } })
          emit({ workspaceId, type: 'run_complete', payload: { data: { session_id: sessionId } } })
        }, 250)
        return { ok: true }
      }
      if (operation === 'cancel') {
        emit({ workspaceId, type: 'run_complete', payload: { data: { session_id: args?.sessionId, cancelled: true } } })
        return { ok: true }
      }
      if (operation === 'config') return config
      if (operation === 'model') {
        const modelType = String(args?.modelType || 'large')
        const model = args?.model
        const sessionId = String(args?.sessionId || '')
        if (sessionId && modelType === 'large' && model) {
          config.session_models = { ...config.session_models, [sessionId]: model }
        } else if (model && (modelType === 'large' || modelType === 'small')) {
          config.models = { ...config.models, [modelType]: model }
        }
        return { ok: true }
      }
      if (operation === 'orchestrator') {
        const prev = config.orchestrator || { enabled: false, final_review: true }
        const nextOrch = { ...prev }
        const sessionId = String(args?.sessionId || '')
        if (args?.enabled !== undefined) {
          if (sessionId) {
            config.session_orchestrator = {
              ...(config.session_orchestrator || {}),
              [sessionId]: Boolean(args.enabled)
            }
          } else {
            nextOrch.enabled = Boolean(args.enabled)
          }
        }
        if (args?.planner) {
          if (!args.planner.provider && !args.planner.model) delete nextOrch.planner
          else nextOrch.planner = { ...args.planner }
        }
        if (args?.executor) {
          if (!args.executor.provider && !args.executor.model) delete nextOrch.executor
          else nextOrch.executor = { ...args.executor }
        }
        if (args?.finalReview !== undefined) {
          nextOrch.final_review = Boolean(args.finalReview)
        }
        config.orchestrator = nextOrch
        emit({ workspaceId, type: 'config_changed', payload: {} })
        return {
          orchestrator: nextOrch,
          session_orchestrator: config.session_orchestrator || {}
        }
      }
      if (operation === 'gamedev') {
        const sessionId = String(args?.sessionId || '')
        const enabled = Boolean(args?.enabled)
        if (sessionId) {
          config.session_game_dev = {
            ...(config.session_game_dev || {}),
            [sessionId]: enabled
          }
        } else {
          config.options = {
            ...(config.options || {}),
            game_dev: enabled
          }
        }
        emit({ workspaceId, type: 'config_changed', payload: {} })
        return {
          game_dev: Boolean(config.options?.game_dev),
          session_game_dev: config.session_game_dev || {}
        }
      }
      if (operation === 'providers') return providers
      if (operation === 'models') {
        const primeModels = accountState.signedIn
          ? [
              {
                provider_id: 'primeai',
                provider_name: 'PrimeAI',
                model_id: '__primeai_auto__',
                name: 'Автоматически',
                context_window: 270000,
                can_reason: true,
                reasoning_modes: ['off', 'low', 'medium', 'high'],
                reasoning_levels: ['low', 'medium', 'high'],
                default_reasoning_mode: 'medium',
                supports_attachments: true
              },
              {
                provider_id: 'primeai',
                provider_name: 'PrimeAI',
                model_id: 'gpt-5.4-pro',
                name: 'GPT-5.4',
                context_window: 270000,
                can_reason: true,
                reasoning_modes: ['off', 'low', 'medium', 'high'],
                reasoning_levels: ['low', 'medium', 'high'],
                default_reasoning_mode: 'medium',
                supports_attachments: true,
                plan_label: 'Pro',
                locked: false,
                allowed_tariffs: ['Pro', 'Ultra']
              },
              {
                provider_id: 'primeai',
                provider_name: 'PrimeAI',
                model_id: 'claude-opus-4.5',
                name: 'Claude Opus 4.5',
                context_window: 200000,
                can_reason: true,
                reasoning_modes: ['off', 'low', 'medium', 'high'],
                reasoning_levels: ['low', 'medium', 'high'],
                default_reasoning_mode: 'medium',
                supports_attachments: true,
                plan_label: 'Ultra',
                locked: true,
                allowed_tariffs: ['Ultra']
              }
            ]
          : []
        return [
          ...primeModels,
          ...providers.flatMap((p) =>
            p.models.map((m) => ({
              provider_id: p.id,
              provider_name: p.name,
              model_id: m.id,
              ...m
            }))
          )
        ]
      }
      if (operation === 'agent') {
        const ws = projects.find((item) => item.id === workspaceId)
        return {
          mode:
            workspaceModes.get(workspaceId) ||
            (ws?.kind === 'designer' ? 'designer' : preferences.agentMode || 'general'),
          model: { name: 'GPT-5.4' }
        }
      }
      if (operation === 'agentWorkers') return { workers }
      if (operation === 'agentWorkerMessages') {
        const workerSessionId =
          args?.workerSessionId ||
          workers.find((item) => item.id === args?.workerId)?.session_id ||
          ''
        const list = workerMessages[workerSessionId] || []
        const limit = Math.max(1, Number(args?.limit ?? 60))
        let slice = list
        let hasBefore = false
        let hasAfter = false
        if (args?.before) {
          const idx = list.findIndex((item) => item.id === args.before)
          const end = idx >= 0 ? idx : list.length
          const start = Math.max(0, end - limit)
          slice = list.slice(start, end)
          hasBefore = start > 0
          hasAfter = end < list.length
        } else if (args?.after) {
          const idx = list.findIndex((item) => item.id === args.after)
          const start = idx >= 0 ? idx + 1 : 0
          const end = Math.min(list.length, start + limit)
          slice = list.slice(start, end)
          hasBefore = start > 0
          hasAfter = end < list.length
        } else if (args?.around) {
          const idx = list.findIndex((item) => item.id === args.around)
          const center = idx >= 0 ? idx : Math.max(0, list.length - 1)
          const half = Math.floor(limit / 2)
          const start = Math.max(0, center - half)
          const end = Math.min(list.length, start + limit)
          slice = list.slice(start, end)
          hasBefore = start > 0
          hasAfter = end < list.length
        } else {
          const start = Math.max(0, list.length - limit)
          slice = list.slice(start)
          hasBefore = start > 0
          hasAfter = false
        }
        return {
          messages: slice,
          has_more: hasBefore || hasAfter,
          has_before: hasBefore,
          has_after: hasAfter,
          timing: { completed: {} }
        }
      }
      if (operation === 'agentWorkerSend') {
        const workerSessionId =
          args?.workerSessionId ||
          workers.find((item) => item.id === args?.workerId)?.session_id ||
          ''
        const list = workerMessages[workerSessionId] || []
        list.push({
          id: `wmsg-${Date.now()}`,
          session_id: workerSessionId,
          role: 'user',
          created_at: Math.floor(Date.now() / 1000),
          parts: [{ type: 'text', data: { text: args?.text || args?.prompt || '' } }]
        })
        workerMessages[workerSessionId] = list
        return { ok: true }
      }
      if (operation === 'mcp') return config.mcp
      if (operation === 'lsp') return config.lsp
      if (operation === 'skills')
        return [
          {
            id: 'prime://skills/prime-config/SKILL.md',
            name: 'prime-config',
            description: 'Builtin Prime configuration helper',
            label: 'system:prime-config',
            source: 'system'
          },
          {
            id: 'prime://skills/prime-designer/SKILL.md',
            name: 'prime-designer',
            description: 'UI craft guidelines',
            label: 'system:prime-designer',
            source: 'system'
          },
          {
            id: 'prime://skills/prime-gamedev/SKILL.md',
            name: 'prime-gamedev',
            description: 'Builtin GameDev QA',
            label: 'system:prime-gamedev',
            source: 'system'
          },
          {
            id: 'C:/Users/preview/.config/prime/skills/custom-workflow/SKILL.md',
            name: 'custom-workflow',
            description: 'Пользовательский навык для релиза проекта.',
            label: 'user:custom-workflow',
            source: 'user'
          }
        ]
      if (operation === 'commands') return []
      if (operation === 'jobs') return jobs
      if (operation === 'queue') return []
      if (operation === 'channelMessages') return []
      if (operation === 'getSkip') return { skip: false }
      if (operation === 'stt') return { provider: 'whisper', models: [] }
      if (operation === 'sttTasks' || operation === 'pluginTasks') return []
      if (operation === 'plugins') return []
      if (operation === 'voice') return {}
      if (operation === 'goal') return { active: false, text: '' }
      if (operation === 'context') return { files: [], rules: [] }
      return {}
    },
    async files(_workspaceId, path = '') {
      return filesTree[path] || []
    },
    async searchFiles(_workspaceId, query = '', limit = 50) {
      const all = Object.values(filesTree).flat()
      const needle = String(query || '').trim().toLowerCase()
      const filtered = needle
        ? all.filter(
            (item) =>
              item.name.toLowerCase().includes(needle) || item.path.toLowerCase().includes(needle)
          )
        : all
      return filtered.slice(0, Math.max(1, limit))
    },
    async saveFile(_workspaceId, path, content) {
      const revision = `rev-${Date.now()}`
      fileContents[path] = { content, revision }
      return { revision }
    },
    async editorDirty() {},
    async readFile(_workspaceId, path) {
      if (isVideoPath(path)) {
        return {
          path,
          name: fileNameFromPath(path),
          video: { path, mime: videoMimeFromPath(path) || 'video/mp4' }
        }
      }
      return fileContents[path] || { content: `// ${path}\n`, revision: 'rev-1' }
    },
    async mediaUrl(_workspaceId, path) {
      const trimmed = String(path || '').trim()
      if (isDirectMediaUrl(trimmed)) {
        return {
          url: trimmed,
          mime: videoMimeFromPath(trimmed),
          name: fileNameFromPath(trimmed)
        }
      }
      if (isVideoPath(trimmed)) {
        return {
          url: previewDemoVideoDataUrl,
          mime: videoMimeFromPath(trimmed) || 'video/mp4',
          name: fileNameFromPath(trimmed)
        }
      }
      if (/\.(png|jpe?g|webp|gif|bmp|avif)$/i.test(trimmed)) {
        const name = fileNameFromPath(trimmed) || 'image.png'
        const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%" stop-color="#1d2432"/><stop offset="100%" stop-color="#11151d"/></linearGradient></defs><rect width="640" height="360" fill="url(#g)"/><rect x="32" y="32" width="576" height="296" rx="14" fill="none" stroke="#ff6b35" stroke-opacity="0.45" stroke-width="2"/><text x="320" y="176" text-anchor="middle" fill="#eef1f5" font-family="system-ui,sans-serif" font-size="22" font-weight="600">${name.replace(/[<>&"]/g, '')}</text><text x="320" y="208" text-anchor="middle" fill="#9ca3af" font-family="monospace" font-size="13">Preview image</text></svg>`
        return {
          url: `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`,
          mime: 'image/png',
          name
        }
      }
      return { url: trimmed, name: fileNameFromPath(trimmed) }
    },
    async inspectPath(_workspaceId, rawPath) {
      const trimmed = String(rawPath || '').trim()
      if (!trimmed || /missing|not-found/i.test(trimmed)) {
        return { exists: false, kind: 'missing' }
      }
      const name = fileNameFromPath(trimmed) || trimmed
      if (isVideoPath(trimmed)) {
        return {
          exists: true,
          kind: 'video',
          mime: videoMimeFromPath(trimmed) || 'video/mp4',
          name,
          path: trimmed,
          previewable: true,
          revealable: true
        }
      }
      if (/\.(png|jpe?g|webp|gif|bmp|avif)$/i.test(trimmed)) {
        return {
          exists: true,
          kind: 'image',
          mime: 'image/png',
          name,
          path: trimmed,
          previewable: true,
          revealable: true
        }
      }
      if (/([\\/]|^[a-zA-Z]:)/.test(trimmed) && !/\.[a-zA-Z0-9]{1,8}$/.test(trimmed)) {
        return {
          exists: true,
          kind: 'dir',
          name,
          path: trimmed,
          previewable: false,
          revealable: true
        }
      }
      return { exists: false, kind: 'missing' }
    },
    async diff(_workspaceId, _sessionId, path) {
      const current = fileContents[path]?.content || ''
      return {
        original: current.replace('workbench = true', 'workbench = false'),
        modified: current,
        content: current
      }
    },
    async attachments() {
      return []
    },
    async importImages(images) {
      return images.map((image, index) => {
        let binary = ''
        for (let offset = 0; offset < image.data.length; offset += 32768) {
          binary += String.fromCharCode(...image.data.subarray(offset, offset + 32768))
        }
        const base64 = btoa(binary)
        const mimeType = image.name.toLowerCase().endsWith('.jpg') || image.name.toLowerCase().endsWith('.jpeg')
          ? 'image/jpeg'
          : image.name.toLowerCase().endsWith('.webp')
            ? 'image/webp'
            : image.name.toLowerCase().endsWith('.gif')
              ? 'image/gif'
              : 'image/png'
        const id = `att-preview-${Date.now()}-${index}`
        const preview = `data:${mimeType};base64,${base64}`
        previewAttachments.set(id, {
          id,
          name: image.name,
          size: image.data.length,
          mimeType,
          base64,
          preview
        })
        return {
          id,
          name: image.name,
          size: image.data.length,
          mimeType,
          preview
        }
      })
    },
    async releaseAttachments(ids) {
      for (const id of ids) previewAttachments.delete(id)
    },
    async preferences(patch) {
      if (patch) {
        preferences = {
          ...preferences,
          ...patch,
          drafts: { ...preferences.drafts, ...patch.drafts },
          skipBySession: { ...preferences.skipBySession, ...patch.skipBySession },
          reasoningBySession: { ...preferences.reasoningBySession, ...patch.reasoningBySession },
          agentContexts: { ...preferences.agentContexts, ...patch.agentContexts },
          sessionOrder: { ...preferences.sessionOrder, ...patch.sessionOrder }
        }
      }
      return structuredClone(preferences)
    },
    async openExternal() {},
    async windowDimmed() {},
    async reveal() {},
    async createEntry(_workspaceId, path) {
      fileContents[path] = { content: '', revision: 'rev-1' }
      return { path }
    },
    async trashEntry(_workspaceId, path) {
      delete fileContents[path]
    },
    async openWith() {},
    async browserShow() {},
    async browserHide() {},
    async browserPreview() {
      return ''
    },
    async browserTab(_action, _id, url) {
      if (url) browserState = { ...browserState, url }
      return browserState
    },
    async browserState() {
      return browserState
    },
    async browserNavigate(url) {
      browserState = { ...browserState, url }
      return browserState
    },
    async browserCommand() {
      return browserState
    },
    async browserOpenExternal() {},
    async browserView() {
      return browserState
    },
    async update() {
      return { state: 'current', version: '0.4.105' }
    },
    account,
    async customAgents(request: CustomAgentsRequest) {
      const nowSec = () => Math.floor(Date.now() / 1000)
      const emitCustom = (
        type: string,
        agent_id: string,
        run_id: string,
        chain_id: string,
        payload: any
      ) => {
        emit({
          workspaceId: '',
          kind: 'customAgents',
          data: { type, agent_id, run_id, chain_id, payload, timestamp: nowSec() }
        })
      }
      const slugify = (name: string, fallback: string) => {
        const cleaned = name
          .toLowerCase()
          .replace(/[^a-z0-9а-яё]+/gi, '-')
          .replace(/^-+|-+$/g, '')
        return cleaned || fallback
      }
      if (request.action === 'list') {
        return { agents: structuredClone(customAgents) }
      }
      if (request.action === 'get') {
        const found = customAgents.find((item) => item.id === request.agentId)
        if (!found) throw new Error('Агент не найден')
        return structuredClone(found)
      }
      if (request.action === 'create') {
        const now = nowSec()
        const id = request.input.id?.trim() || `agent-${Date.now()}`
        const name = request.input.name?.trim() || 'Новый агент'
        const created: CustomAgent = {
          id,
          slug: request.input.slug?.trim() || slugify(name, id),
          name,
          description: request.input.description || '',
          role: request.input.role || 'worker',
          system_prompt: request.input.system_prompt || '',
          memory: request.input.memory || '',
          model_mode: request.input.model_mode || 'auto',
          provider_id: request.input.provider_id || '',
          model_id: request.input.model_id || '',
          reasoning_mode: request.input.reasoning_mode || '',
          auto_model_hint: request.input.auto_model_hint || '',
          allowed_tools: request.input.allowed_tools || [],
          workspace_path: request.input.workspace_path || '',
          workflow_id: request.input.workflow_id || 'default',
          enabled: request.input.enabled ?? true,
          created_by: request.input.created_by || 'user',
          trigger_type: request.input.trigger_type || 'manual',
          interval_minutes: request.input.interval_minutes || 0,
          cron_expr: request.input.cron_expr || '',
          monitor_kind: request.input.monitor_kind || '',
          monitor_target: request.input.monitor_target || '',
          monitor_last_hash: request.input.monitor_last_hash || '',
          monitor_last_snapshot: request.input.monitor_last_snapshot || '',
          acceptance_criteria: request.input.acceptance_criteria || '',
          deterministic_checks: request.input.deterministic_checks || [],
          verifier_agent_id: request.input.verifier_agent_id || '',
          next_agent_id: request.input.next_agent_id || '',
          max_rework_attempts: request.input.max_rework_attempts || 3,
          strict_verify: request.input.strict_verify ?? false,
          status: request.input.enabled === false ? 'paused' : 'idle',
          last_error: '',
          last_run_at: 0,
          next_run_at: 0,
          created_at: now,
          updated_at: now
        }
        customAgents = [...customAgents, created]
        emitCustom('agent.created', created.id, '', '', structuredClone(created))
        return structuredClone(created)
      }
      if (request.action === 'update') {
        const index = customAgents.findIndex((item) => item.id === request.agentId)
        if (index === -1) throw new Error('Агент не найден')
        const current = customAgents[index]
        const nextEnabled =
          request.patch.enabled !== undefined ? request.patch.enabled : current.enabled
        const nextStatus =
          request.patch.enabled === false
            ? 'paused'
            : request.patch.enabled === true && current.status === 'paused'
              ? 'idle'
              : request.patch.status || current.status
        const updated: CustomAgent = {
          ...current,
          ...request.patch,
          id: current.id,
          enabled: nextEnabled,
          status: nextStatus,
          updated_at: nowSec()
        }
        customAgents = customAgents.map((item) => (item.id === updated.id ? updated : item))
        emitCustom('agent.status', updated.id, '', '', structuredClone(updated))
        return structuredClone(updated)
      }
      if (request.action === 'delete') {
        const exists = customAgents.some((item) => item.id === request.agentId)
        customAgents = customAgents.filter((item) => item.id !== request.agentId)
        return { deleted: exists }
      }
      if (request.action === 'probe') {
        const index = customAgents.findIndex((item) => item.id === request.agentId)
        if (index === -1) throw new Error('Агент не найден')
        const agent = customAgents[index]
        if (!agent.monitor_kind || !agent.monitor_target) {
          return {
            changed: false,
            hash: agent.monitor_last_hash,
            snapshot: agent.monitor_last_snapshot,
            diff: '',
            error: 'Не задан тип или цель мониторинга'
          }
        }
        const nextVersion = agent.monitor_last_snapshot.includes('v0.4.118')
          ? 'v0.4.119'
          : 'v0.4.118'
        const nextSnapshot =
          agent.monitor_kind === 'http'
            ? `{"tag_name":"${nextVersion}","draft":false,"checked_at":${nowSec()}}`
            : `snapshot:${agent.monitor_target}:${nextVersion}`
        const nextHash = Array.from(nextSnapshot)
          .reduce((acc, ch) => (acc * 33 + ch.charCodeAt(0)) >>> 0, 5381)
          .toString(16)
          .padStart(16, '0')
        const diff = `--- previous\n+++ current\n- ${agent.monitor_last_snapshot || '(пусто)'}\n+ ${nextSnapshot}`
        const updated = {
          ...agent,
          monitor_last_hash: nextHash,
          monitor_last_snapshot: nextSnapshot,
          updated_at: nowSec()
        }
        customAgents = customAgents.map((item) => (item.id === updated.id ? updated : item))
        emitCustom('agent.status', updated.id, '', '', structuredClone(updated))
        return {
          changed: nextHash !== agent.monitor_last_hash,
          hash: nextHash,
          snapshot: nextSnapshot,
          diff,
          error: ''
        }
      }
      if (request.action === 'run') {
        const agent = customAgents.find((item) => item.id === request.agentId)
        if (!agent) throw new Error('Агент не найден')
        const started = nowSec()
        const chainId = request.chainId?.trim() || `chain-${Date.now()}`
        const runId = `crun-${Date.now()}`
        const chosen =
          agent.model_mode === 'manual' && agent.model_id
            ? customModels.find(
                (m) => m.provider_id === agent.provider_id && m.model_id === agent.model_id
              ) || customModels[1]
            : agent.role === 'orchestrator'
              ? customModels[3]
              : agent.role === 'verifier' || agent.acceptance_criteria
                ? customModels[1]
                : customModels[0]
        const resolvedReasoning =
          agent.model_mode === 'manual' && agent.reasoning_mode
            ? agent.reasoning_mode
            : chosen.default_reasoning_mode || 'medium'
        const autoReason =
          agent.model_mode === 'manual'
            ? `Выбрано вручную (рассуждения: ${resolvedReasoning}).`
            : `Автовыбор (${chosen.tier}, рассуждения: ${resolvedReasoning}): подобрано под роль и критерии задачи.`
        const initialRun: CustomAgentRun = {
          id: runId,
          agent_id: agent.id,
          agent_name: agent.name,
          agent_role: agent.role,
          chain_id: chainId,
          parent_run_id: '',
          trigger_reason: request.triggerReason || 'manual',
          input_prompt: request.prompt,
          status: 'running',
          attempt: 1,
          resolved_provider: chosen.provider_id,
          resolved_model: chosen.model_id,
          resolved_reasoning: resolvedReasoning,
          auto_model_reason: autoReason,
          output_text: '',
          verifier_verdict: '',
          verifier_feedback: '',
          deterministic_report: '',
          monitor_diff: '',
          steps: [
            {
              kind: 'execute',
              title: 'Запуск задачи',
              detail: request.prompt.slice(0, 120) || 'Выполнение по инструкции',
              status: 'running',
              timestamp: started
            }
          ],
          error: '',
          started_at: started,
          finished_at: 0
        }
        customAgentRuns = [initialRun, ...customAgentRuns]
        const finalizeRun = (): CustomAgentRun => {
          const doneAt = nowSec()
          const needsVerify =
            agent.role !== 'verifier' &&
            (Boolean(agent.strict_verify) ||
              Boolean(agent.acceptance_criteria.trim()) ||
              agent.deterministic_checks.length > 0 ||
              Boolean(agent.verifier_agent_id))
          const outputText =
            agent.role === 'orchestrator'
              ? `Задача принята координатором:\n> ${request.prompt}\n\nПлан выполнен: требования зафиксированы на доске контекста, параметры моделей согласованы.`
              : `Результат выполнения агента «${agent.name}»:\n\n${request.prompt ? `Запрос: ${request.prompt}\n` : ''}Статус: выполнено успешно.`
          const completed: CustomAgentRun = {
            ...initialRun,
            status: 'passed',
            output_text: outputText,
            verifier_verdict: needsVerify ? 'PASS' : '',
            verifier_feedback: needsVerify
              ? 'VERDICT: PASS\nКритерии приёмки и структурные проверки выполнены.'
              : '',
            deterministic_report: agent.deterministic_checks.length
              ? agent.deterministic_checks
                  .map((chk) => `[PASS] ${chk.id || chk.kind} (${chk.kind}): проверка пройдена`)
                  .join('\n')
              : '',
            steps: [
              {
                kind: 'execute',
                title: 'Выполнение задачи',
                detail: 'Сформирован итоговый ответ',
                status: 'ok',
                timestamp: doneAt
              },
              ...(needsVerify
                ? [
                    {
                      kind: 'verifier',
                      title: 'Верификация пройдена',
                      detail: 'VERDICT: PASS',
                      status: 'ok',
                      timestamp: doneAt
                    }
                  ]
                : [])
            ],
            finished_at: doneAt
          }
          customAgentRuns = customAgentRuns.map((r) => (r.id === runId ? completed : r))
          const updatedAgent: CustomAgent = {
            ...agent,
            status: agent.enabled ? 'idle' : 'paused',
            last_run_at: doneAt,
            updated_at: doneAt
          }
          customAgents = customAgents.map((a) => (a.id === agent.id ? updatedAgent : a))
          if (agent.next_agent_id) {
            const handoff: BlackboardEntry = {
              id: `bb-${Date.now()}`,
              chain_id: chainId,
              run_id: runId,
              from_agent_id: agent.id,
              from_agent_name: agent.name,
              to_agent_id: agent.next_agent_id,
              topic: 'handoff',
              payload: outputText,
              status: 'open',
              created_at: doneAt
            }
            customBlackboard = [handoff, ...customBlackboard]
            emitCustom('blackboard.posted', agent.id, runId, chainId, structuredClone(handoff))
          }
          emitCustom('agent.status', agent.id, '', '', structuredClone(updatedAgent))
          emitCustom('run.passed', agent.id, runId, chainId, structuredClone(completed))
          return completed
        }
        if (request.sync) {
          const completed = finalizeRun()
          return structuredClone(completed)
        }
        const runningAgent: CustomAgent = { ...agent, status: 'running', updated_at: started }
        customAgents = customAgents.map((a) => (a.id === agent.id ? runningAgent : a))
        emitCustom('agent.status', agent.id, '', '', structuredClone(runningAgent))
        emitCustom('run.started', agent.id, runId, chainId, structuredClone(initialRun))
        const t1 = setTimeout(() => {
          const step = {
            kind: 'execute',
            title: 'Анализ и подготовка ответа',
            detail: 'Обработка контекста и памяти агента',
            status: 'ok',
            timestamp: nowSec()
          }
          customAgentRuns = customAgentRuns.map((r) =>
            r.id === runId ? { ...r, steps: [...r.steps, step] } : r
          )
          emitCustom('run.step', agent.id, runId, chainId, step)
        }, 120)
        const t2 = setTimeout(() => {
          customRunTimers.delete(runId)
          finalizeRun()
        }, 280)
        customRunTimers.set(runId, [t1, t2])
        return structuredClone(initialRun)
      }
      if (request.action === 'cancelRun') {
        const timers = customRunTimers.get(request.runId)
        if (timers) {
          for (const timer of timers) clearTimeout(timer)
          customRunTimers.delete(request.runId)
        }
        const run = customAgentRuns.find((r) => r.id === request.runId)
        if (!run) return { cancelled: false }
        const cancelled: CustomAgentRun = {
          ...run,
          status: 'cancelled',
          error: 'выполнение отменено',
          finished_at: nowSec()
        }
        customAgentRuns = customAgentRuns.map((r) => (r.id === request.runId ? cancelled : r))
        const agent = customAgents.find((a) => a.id === request.agentId)
        if (agent) {
          const nextAgent: CustomAgent = {
            ...agent,
            status: agent.enabled ? 'idle' : 'paused',
            updated_at: nowSec()
          }
          customAgents = customAgents.map((a) => (a.id === nextAgent.id ? nextAgent : a))
          emitCustom('agent.status', nextAgent.id, '', '', structuredClone(nextAgent))
        }
        emitCustom(
          'run.cancelled',
          request.agentId,
          request.runId,
          cancelled.chain_id,
          structuredClone(cancelled)
        )
        return { cancelled: true }
      }
      if (request.action === 'runs') {
        const filtered = request.agentId
          ? customAgentRuns.filter((r) => r.agent_id === request.agentId)
          : customAgentRuns
        const sorted = [...filtered].sort((a, b) => b.started_at - a.started_at)
        const limited = request.limit ? sorted.slice(0, request.limit) : sorted
        return { runs: structuredClone(limited) }
      }
      if (request.action === 'blackboard') {
        const filtered = request.chainId
          ? customBlackboard.filter((b) => b.chain_id === request.chainId)
          : customBlackboard
        const sorted = [...filtered].sort((a, b) => b.created_at - a.created_at)
        const limited = request.limit ? sorted.slice(0, request.limit) : sorted
        return { entries: structuredClone(limited) }
      }
      if (request.action === 'postBlackboard') {
        const entry: BlackboardEntry = {
          id: `bb-${Date.now()}`,
          chain_id: request.input.chain_id,
          run_id: '',
          from_agent_id: request.input.from_agent_id || '',
          from_agent_name: request.input.from_agent_name || 'Пользователь',
          to_agent_id: request.input.to_agent_id || '',
          topic: request.input.topic,
          payload: request.input.payload,
          status: 'open',
          created_at: nowSec()
        }
        customBlackboard = [entry, ...customBlackboard]
        emitCustom('blackboard.posted', entry.from_agent_id, '', entry.chain_id, structuredClone(entry))
        return structuredClone(entry)
      }
      if (request.action === 'models') {
        return { models: structuredClone(customModels) }
      }
      if (request.action === 'getGlobalMemory') {
        return structuredClone(globalMemory)
      }
      if (request.action === 'saveGlobalMemory') {
        const input = request.input || {}
        const memory = typeof input.memory === 'string' ? input.memory : globalMemory.memory
        const user_profile =
          typeof input.user_profile === 'string' ? input.user_profile : globalMemory.user_profile
        const soul = typeof input.soul === 'string' ? input.soul : globalMemory.soul
        const memory_chars = Array.from(memory).length
        const user_chars = Array.from(user_profile).length
        if (memory_chars > globalMemory.memory_limit) {
          throw new Error(
            `Превышен лимит MEMORY.md (${memory_chars} / ${globalMemory.memory_limit} символов)`
          )
        }
        if (user_chars > globalMemory.user_limit) {
          throw new Error(
            `Превышен лимит USER.md (${user_chars} / ${globalMemory.user_limit} символов)`
          )
        }
        globalMemory = {
          ...globalMemory,
          memory,
          user_profile,
          soul,
          memory_chars,
          user_chars
        }
        return structuredClone(globalMemory)
      }
      if (request.action === 'dispatchSwarm') {
        const req = request.input
        const goal = req.goal?.trim() || ''
        if (!goal) {
          throw new Error('Требуется цель роя')
        }
        const specs = Array.isArray(req.workers) ? req.workers : []
        if (specs.length === 0) {
          throw new Error('Требуется хотя бы один агент в рое')
        }
        const now = nowSec()
        const stamp = Date.now()
        const chainId = req.chain_id?.trim() || `swarm-${stamp}`
        const createdBoard: BlackboardEntry[] = []
        const pushBoard = (entry: BlackboardEntry) => {
          createdBoard.push(entry)
          customBlackboard = [entry, ...customBlackboard]
          emitCustom('blackboard.posted', entry.from_agent_id, entry.run_id, chainId, structuredClone(entry))
        }

        pushBoard({
          id: `bb-${stamp}-goal`,
          chain_id: chainId,
          run_id: '',
          from_agent_id: 'swarm-coordinator',
          from_agent_name: 'Координатор роя',
          to_agent_id: '',
          topic: 'swarm.goal',
          payload: `Цель: ${goal}\nВоркеры: ${specs.length}`,
          status: 'open',
          created_at: now
        })

        emitCustom('swarm.started', '', '', chainId, { goal, workers: specs.length })

        const workerResults: SwarmWorkerResult[] = specs.map((spec, index) => {
          const matched =
            (spec.agent_id && customAgents.find((a) => a.id === spec.agent_id)) ||
            (spec.slug && customAgents.find((a) => a.slug === spec.slug)) ||
            null
          const agentId = matched?.id || spec.agent_id?.trim() || `swarm-worker-${index + 1}`
          const slug = matched?.slug || spec.slug?.trim() || `swarm-worker-${index + 1}`
          const name = matched?.name || spec.name?.trim() || slug
          const role = matched?.role || spec.role || 'worker'
          const task = spec.task?.trim() || spec.prompt?.trim() || goal
          const chosen =
            matched?.model_mode === 'manual' && matched.model_id
              ? customModels.find(
                  (m) => m.provider_id === matched.provider_id && m.model_id === matched.model_id
                ) || customModels[1]
              : role === 'orchestrator'
                ? customModels[3]
                : customModels[0]
          const reasoning = matched?.reasoning_mode || chosen.default_reasoning_mode || 'medium'
          const runId = `crun-${stamp}-w${index + 1}`
          const output = `Воркер «${name}» выполнил подзадачу: ${task}`
          const run: CustomAgentRun = {
            id: runId,
            agent_id: agentId,
            agent_name: name,
            agent_role: role,
            chain_id: chainId,
            parent_run_id: '',
            trigger_reason: 'swarm:worker',
            input_prompt: task,
            status: 'passed',
            attempt: 1,
            resolved_provider: chosen.provider_id,
            resolved_model: chosen.model_id,
            resolved_reasoning: reasoning,
            auto_model_reason: `Автовыбор (${chosen.tier}, рассуждения: ${reasoning}) для параллельного роя.`,
            output_text: output,
            verifier_verdict: '',
            verifier_feedback: '',
            deterministic_report: '',
            monitor_diff: '',
            steps: [
              {
                kind: 'execute',
                title: 'Параллельное выполнение',
                detail: task.slice(0, 120),
                status: 'ok',
                timestamp: now
              }
            ],
            error: '',
            started_at: now,
            finished_at: now
          }
          customAgentRuns = [run, ...customAgentRuns]
          emitCustom('run.passed', agentId, runId, chainId, structuredClone(run))

          pushBoard({
            id: `bb-${stamp}-w${index + 1}`,
            chain_id: chainId,
            run_id: runId,
            from_agent_id: agentId,
            from_agent_name: name,
            to_agent_id: '',
            topic: 'swarm.worker',
            payload: output,
            status: 'open',
            created_at: now
          })

          return {
            agent_id: agentId,
            slug,
            name,
            task,
            run,
            output,
            status: 'passed'
          }
        })

        let verifierVerdict = ''
        let verifierFeedback = ''
        if (!req.skip_verify) {
          const verifierAgent =
            (req.verifier_agent_id && customAgents.find((a) => a.id === req.verifier_agent_id)) ||
            customAgents.find((a) => a.role === 'verifier' && a.enabled) ||
            null
          verifierVerdict = 'PASS'
          verifierFeedback = req.acceptance_criteria?.trim()
            ? `VERDICT: PASS\nКритерии приёмки подтверждены для ${workerResults.length} воркеров.`
            : `VERDICT: PASS\nРезультаты ${workerResults.length} воркеров проверены и согласованы.`
          pushBoard({
            id: `bb-${stamp}-verdict`,
            chain_id: chainId,
            run_id: '',
            from_agent_id: verifierAgent?.id || 'swarm-verifier',
            from_agent_name: verifierAgent?.name || 'Верификатор роя',
            to_agent_id: '',
            topic: 'swarm.verdict',
            payload: verifierFeedback,
            status: 'open',
            created_at: now
          })
        }

        let finalOutput = ''
        let synthesizerRun: CustomAgentRun | null = null
        if (req.skip_synthesize) {
          finalOutput = workerResults.map((w) => `• ${w.name}: ${w.output}`).join('\n')
        } else {
          const synthAgent =
            (req.synthesizer_agent_id &&
              customAgents.find((a) => a.id === req.synthesizer_agent_id)) ||
            customAgents.find((a) => a.role === 'orchestrator' && a.enabled) ||
            null
          finalOutput = [
            `Итог роя по цели: ${goal}`,
            '',
            ...workerResults.map((w) => `- ${w.name}: ${w.output}`)
          ].join('\n')
          const synthId = `crun-${stamp}-synth`
          const synthModel = customModels[3] || customModels[1]
          synthesizerRun = {
            id: synthId,
            agent_id: synthAgent?.id || 'swarm-synthesizer',
            agent_name: synthAgent?.name || 'Синтезатор роя',
            agent_role: synthAgent?.role || 'orchestrator',
            chain_id: chainId,
            parent_run_id: '',
            trigger_reason: 'swarm:synthesize',
            input_prompt: goal,
            status: 'passed',
            attempt: 1,
            resolved_provider: synthModel.provider_id,
            resolved_model: synthModel.model_id,
            resolved_reasoning: synthModel.default_reasoning_mode || 'high',
            auto_model_reason: `Автовыбор (${synthModel.tier}) для синтеза результатов роя.`,
            output_text: finalOutput,
            verifier_verdict: verifierVerdict,
            verifier_feedback: verifierFeedback,
            deterministic_report: '',
            monitor_diff: '',
            steps: [
              {
                kind: 'execute',
                title: 'Синтез результатов роя',
                detail: `Объединено результатов: ${workerResults.length}`,
                status: 'ok',
                timestamp: now
              }
            ],
            error: '',
            started_at: now,
            finished_at: now
          }
          customAgentRuns = [synthesizerRun, ...customAgentRuns]
          pushBoard({
            id: `bb-${stamp}-final`,
            chain_id: chainId,
            run_id: synthId,
            from_agent_id: synthesizerRun.agent_id,
            from_agent_name: synthesizerRun.agent_name,
            to_agent_id: '',
            topic: 'swarm.final',
            payload: finalOutput,
            status: 'open',
            created_at: now
          })
        }

        const swarmResult: SwarmResult = {
          chain_id: chainId,
          goal,
          status: 'passed',
          attempts: 1,
          workers: workerResults,
          verifier_verdict: verifierVerdict,
          verifier_feedback: verifierFeedback,
          deterministic_report: '',
          final_output: finalOutput,
          synthesizer_run: synthesizerRun,
          blackboard: [...createdBoard].reverse()
        }
        emitCustom('swarm.completed', '', '', chainId, structuredClone(swarmResult))
        return structuredClone(swarmResult)
      }
      if (request.action === 'listWorkflows') {
        return { workflows: structuredClone(customWorkflows) }
      }
      if (request.action === 'createWorkflow') {
        const now = nowSec()
        const name = request.input.name?.trim() || request.input.slug?.trim() || ''
        if (!name) throw new Error('Требуется название цепочки')
        const id = `wf-${Date.now()}-${Math.floor(Math.random() * 1000)}`
        const slug = request.input.slug?.trim() || slugify(name, id)
        const created: CustomWorkflow = {
          id,
          slug,
          name,
          description: request.input.description || '',
          enabled: request.input.enabled ?? true,
          created_by: request.input.created_by || 'user',
          entry_agent_id: request.input.entry_agent_id || '',
          layout_json: request.input.layout_json || '',
          created_at: now,
          updated_at: now
        }
        customWorkflows = [...customWorkflows, created]
        emit({
          workspaceId: '',
          kind: 'customAgents',
          data: {
            type: 'workflow_updated',
            agent_id: '',
            run_id: '',
            chain_id: '',
            workflow_id: created.id,
            workflow: structuredClone(created),
            payload: structuredClone(created),
            timestamp: now
          }
        })
        return structuredClone(created)
      }
      if (request.action === 'updateWorkflow') {
        const idx = customWorkflows.findIndex(
          (w) => w.id === request.workflowId || w.slug === request.workflowId
        )
        if (idx === -1) throw new Error('Цепочка не найдена')
        const current = customWorkflows[idx]
        const nextName =
          request.patch.name !== undefined ? request.patch.name.trim() : current.name
        if (!nextName) throw new Error('Название цепочки не может быть пустым')
        const now = nowSec()
        const updated: CustomWorkflow = {
          ...current,
          name: nextName,
          description:
            request.patch.description !== undefined
              ? request.patch.description
              : current.description,
          enabled:
            request.patch.enabled !== undefined ? request.patch.enabled : current.enabled,
          entry_agent_id:
            request.patch.entry_agent_id !== undefined
              ? request.patch.entry_agent_id
              : current.entry_agent_id,
          layout_json:
            request.patch.layout_json !== undefined
              ? request.patch.layout_json
              : current.layout_json,
          updated_at: now
        }
        customWorkflows = customWorkflows.map((w) => (w.id === updated.id ? updated : w))
        emit({
          workspaceId: '',
          kind: 'customAgents',
          data: {
            type: 'workflow_updated',
            agent_id: '',
            run_id: '',
            chain_id: '',
            workflow_id: updated.id,
            workflow: structuredClone(updated),
            payload: structuredClone(updated),
            timestamp: now
          }
        })
        return structuredClone(updated)
      }
      if (request.action === 'deleteWorkflow') {
        const existing = customWorkflows.find(
          (w) => w.id === request.workflowId || w.slug === request.workflowId
        )
        if (!existing) throw new Error('Цепочка не найдена')
        if (existing.id === 'default') {
          throw new Error('Базовую цепочку нельзя удалить')
        }
        const deleteAgents = Boolean(request.deleteAgents)
        if (deleteAgents) {
          customAgents = customAgents.filter(
            (a) => (a.workflow_id || 'default') !== existing.id
          )
        } else {
          customAgents = customAgents.map((a) =>
            (a.workflow_id || 'default') === existing.id ? { ...a, workflow_id: '' } : a
          )
        }
        customWorkflows = customWorkflows.filter((w) => w.id !== existing.id)
        emit({
          workspaceId: '',
          kind: 'customAgents',
          data: {
            type: 'workflow_deleted',
            agent_id: '',
            run_id: '',
            chain_id: '',
            workflow_id: existing.id,
            payload: { workflow_id: existing.id, delete_agents: deleteAgents },
            timestamp: nowSec()
          }
        })
        return { deleted: true }
      }
      if (request.action === 'runWorkflow') {
        const wf = customWorkflows.find(
          (w) => w.id === request.workflowId || w.slug === request.workflowId
        )
        if (!wf) throw new Error('Цепочка не найдена')
        if (!wf.enabled) throw new Error(`Цепочка ${wf.slug} отключена`)
        const wfAgents = customAgents.filter(
          (a) => (a.workflow_id || 'default') === wf.id
        )
        if (wfAgents.length === 0) {
          throw new Error(`В цепочке «${wf.name}» нет агентов для запуска`)
        }
        const incoming = new Set<string>()
        for (const item of wfAgents) {
          if (item.next_agent_id && item.next_agent_id !== item.id) {
            incoming.add(item.next_agent_id)
          }
        }
        const entry =
          (wf.entry_agent_id &&
            wfAgents.find(
              (a) => a.id === wf.entry_agent_id || a.slug === wf.entry_agent_id
            )) ||
          wfAgents.find((a) => a.enabled && !incoming.has(a.id)) ||
          wfAgents.find((a) => a.enabled) ||
          wfAgents[0]
        if (!entry.enabled) {
          throw new Error(`Стартовый агент ${entry.name} отключён`)
        }
        const started = nowSec()
        const chainId = `wf-${wf.slug}-${Date.now()}`
        const runId = `crun-${Date.now()}`
        const chosen = customModels[0]
        const run: CustomAgentRun = {
          id: runId,
          agent_id: entry.id,
          agent_name: entry.name,
          agent_role: entry.role,
          chain_id: chainId,
          parent_run_id: '',
          trigger_reason: `workflow:${wf.slug}`,
          input_prompt: request.prompt || '',
          status: 'passed',
          attempt: 1,
          resolved_provider: chosen.provider_id,
          resolved_model: chosen.model_id,
          resolved_reasoning: chosen.default_reasoning_mode || 'medium',
          auto_model_reason: `Запуск цепочки «${wf.name}» со стартового узла «${entry.name}».`,
          output_text: `Цепочка «${wf.name}» выполнена (старт: ${entry.name}).`,
          verifier_verdict: entry.strict_verify ? 'PASS' : '',
          verifier_feedback: entry.strict_verify ? 'VERDICT: PASS' : '',
          deterministic_report: '',
          monitor_diff: '',
          steps: [
            {
              kind: 'execute',
              title: `Старт цепочки «${wf.name}»`,
              detail: entry.name,
              status: 'ok',
              timestamp: started
            }
          ],
          error: '',
          started_at: started,
          finished_at: started
        }
        customAgentRuns = [run, ...customAgentRuns]
        emitCustom('run.passed', entry.id, runId, chainId, structuredClone(run))
        return structuredClone(run)
      }
      return {}
    },
    onEvent(listener) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    }
  }

  window.prime = api
  setTimeout(() => {
    notifyWarning(
      'Предпросмотр интерфейса',
      'Локальный предпросмотр: изменения сохраняются только в памяти вкладки.'
    )
  }, 400)
}
