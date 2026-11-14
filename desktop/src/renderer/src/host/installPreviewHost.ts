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
  createPreviewBrowserLogs,
  createPreviewBrowserState,
  createPreviewConfig,
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
  createPreviewWorkers
} from './previewHostData'

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
              usedRequests: 120,
              limitRequests: 120,
              percent: 100
            },
            weekly: {
              usedTokens: 4_750_000,
              limitTokens: 5_000_000,
              usedRequests: 950,
              limitRequests: 1000,
              percent: 95
            }
          }
        : limitsMode === 'high'
          ? {
              fiveHour: {
                usedTokens: 410_000,
                limitTokens: 500_000,
                usedRequests: 98,
                limitRequests: 120,
                percent: 82
              },
              weekly: {
                usedTokens: 3_200_000,
                limitTokens: 5_000_000,
                usedRequests: 640,
                limitRequests: 1000,
                percent: 64
              }
            }
          : {
              fiveHour: {
                usedTokens: 142_500,
                limitTokens: 500_000,
                usedRequests: 18,
                limitRequests: 120,
                percent: 28.5
              },
              weekly: {
                usedTokens: 710_000,
                limitTokens: 5_000_000,
                usedRequests: 86,
                limitRequests: 1000,
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
    async projectCreationBase() {
      return 'C:\\Users\\demo\\Projects'
    },
    async createProject(name) {
      const created: Project = {
        id: `ws-${Date.now()}`,
        kind: 'project',
        path: `C:\\Users\\demo\\Projects\\${name}`,
        clientId: 'preview-client',
        connection: 'ready'
      }
      projects = [...projects, created]
      sessionsByProject[created.id] = []
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
      if (operation === 'send') {
        const sessionId = args?.sessionId
        const userMsg = {
          id: `msg-u-${Date.now()}`,
          session_id: sessionId,
          role: 'user',
          created_at: Math.floor(Date.now() / 1000),
          parts: [{ type: 'text', data: { text: args?.prompt || '' } }]
        }
        messagesBySession[sessionId] = [...(messagesBySession[sessionId] || []), userMsg]
        emit({ workspaceId, type: 'message', payload: { action: 'created', data: userMsg } })
        setTimeout(() => {
          const reply = {
            id: `msg-a-${Date.now()}`,
            session_id: sessionId,
            role: 'assistant',
            created_at: Math.floor(Date.now() / 1000),
            parts: [
              {
                type: 'text',
                data: {
                  text: `Готово! Получен запрос в режиме предпросмотра:\n\n> ${String(args?.prompt || '').slice(0, 140)}`
                }
              },
              { type: 'finish', data: { reason: 'stop' } }
            ]
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
      if (operation === 'providers') return providers
      if (operation === 'models') return providers.flatMap((p) => p.models.map((m) => ({ provider: p.id, ...m })))
      if (operation === 'agent') return { mode: preferences.agentMode || 'general', model: { name: 'GPT-5.4' } }
      if (operation === 'agentWorkers') return { workers }
      if (operation === 'agentWorkerMessages') return { messages: workerMessages[args?.workerSessionId] || [] }
      if (operation === 'agentWorkerSend') {
        const list = workerMessages[args?.workerSessionId] || []
        list.push({
          id: `wmsg-${Date.now()}`,
          session_id: args?.workerSessionId,
          role: 'user',
          created_at: Math.floor(Date.now() / 1000),
          parts: [{ type: 'text', data: { text: args?.prompt || '' } }]
        })
        workerMessages[args?.workerSessionId] = list
        return { ok: true }
      }
      if (operation === 'mcp') return config.mcp
      if (operation === 'lsp') return config.lsp
      if (operation === 'skills') return [{ name: 'prime-designer', description: 'UI craft guidelines', enabled: true }]
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
    async saveFile(_workspaceId, path, content) {
      const revision = `rev-${Date.now()}`
      fileContents[path] = { content, revision }
      return { revision }
    },
    async editorDirty() {},
    async readFile(_workspaceId, path) {
      return fileContents[path] || { content: `// ${path}\n`, revision: 'rev-1' }
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
    async importImages() {
      return []
    },
    async releaseAttachments() {},
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
