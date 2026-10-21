import { applyWindowChrome, windowChromeColors } from './windowChrome'
import { UpdateController } from './updates/controller'
import { discoverDesktopRelease } from './updates/github'
import type { UpdateStatus } from '../shared/updates'
import { translate } from '../shared/localization'
import {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  globalShortcut,
  Menu,
  nativeImage,
  nativeTheme,
  Notification,
  session,
  shell,
  Tray
} from 'electron'
import { join, basename, extname } from 'node:path'
import { mkdir, readFile, stat } from 'node:fs/promises'
import { randomUUID } from 'node:crypto'
import { z } from 'zod'
import { RuntimeSupervisor } from './runtime/supervisor'
import { PrimeService } from './prime/service'
import { SettingsStore } from './settings/store'
import { resolveProjectPath } from './files/paths'
import { projectFiles } from './workbench/projectFiles'
import { WorkspaceRouter } from './workbench/workspaceRouter'
import { isProjectLocked, openWhenUnlocked } from './prime/projectLock'
import { RemoteWorkspaces } from './remote/workspaces'
import { createCredentialVault } from './remote/credentialVault'
import { createSshExecutionTools } from './remote/execution'
import { SshAccess, sshProfileTools } from './remote/access'
import { sshUploadTools } from './remote/upload'
import { remoteFingerprint } from './remote/fingerprint'
import { remoteRequest } from './remote/ipc'
import { isRemoteWorkspace, remoteWorkspaceIdentity } from '../shared/remoteWorkspace'
import { BrowserPanel } from './browser/panel'
import { cancelPick, pickElements } from './browser/picker'
import { openInSystemBrowser } from './browser/external'
import { startBrowserMcpServer, type BrowserMcpServer } from './browser/mcp-server'
import { validateExternalUrl } from './ipc/security'
import type { Operation } from '../shared/contracts'
import { TerminalManager } from './terminal/manager'
import { terminalOperation } from './workbench/terminalIpc'
import { workbenchTerminalTools } from './workbench/tools'
import { createProjectTools } from './workbench/projectTools'
import { createGithubReadTools, createGitHistoryTools } from './workbench/projectGithubTools'
import { AutomationManager } from './automations/manager'
import { automationOperation } from './workbench/automationIpc'
import { createAgentAdapter } from './workbench/agentRuns'
import { TeamManager } from './teams/manager'
import { LaunchManager } from './launch/manager'
import { SpeechService } from './speech/service'
import { githubOperation } from './github/service'
import { localRunner, remoteRunner } from './github/runner'
import { WindowViewer } from './computer/viewer'
import { JobCenter } from './jobs/center'
import { teamOperation } from './teams/ipc'
import { ProjectChanges } from './workbench/projectChanges'
import { importImages } from './attachments/images'

let window: BrowserWindow | null = null
let tray: Tray | null = null
let quitting = false
let cleanupDone = false
let cleanupRunning = false
let automations: AutomationManager | undefined
let teams: TeamManager | undefined
const projectChanges = new ProjectChanges()
let runtime: RuntimeSupervisor
let prime: PrimeService | undefined
let remotes: RemoteWorkspaces | undefined
let projectWait: AbortController | undefined
let settings: SettingsStore
let runtimeError = ''
let updateStatus: UpdateStatus = { state: 'idle' }
let updateController: Promise<UpdateController> | undefined
let recovering = false
let editorDirty = false
const t = (source: string) => translate(settings?.get().language, source)
const restarts: number[] = []
const attachmentStore = new Map<string, any>()
const projectId = z.string().uuid()
let browserMcp: BrowserMcpServer | undefined
const emit = (event: any) => {
  if (event.type === 'terminal') {
    event.payload.activeCount = terminals.list().filter((item) => item.status === 'running').length
    launches?.terminalEvent(event.payload)
  }
  if (
    event.type === 'run_complete' &&
    event.payload?.data?.error &&
    !event.payload.data.cancelled &&
    !/context canceled/i.test(event.payload.data.error)
  ) {
    const root = router()?.projects.get(event.workspaceId)?.path
    if (root)
      void automations
        ?.event({ root, type: 'runFailed', sourceRunId: event.payload.data.run_id })
        .catch(() => {})
  }
  if (window && !window.isDestroyed()) window.webContents.send('prime:event', event)
  if (
    event.type === 'runtime' &&
    event.payload?.status === 'failed' &&
    prime &&
    !quitting &&
    !recovering
  )
    void recoverBackend()
  if (event.type === 'run_complete' && window && !window.isFocused()) {
    new Notification({
      title: 'Prime Desktop',
      body: event.payload?.data?.error ? t('Задача завершилась с ошибкой') : t('Задача завершена')
    })
      .on('click', () => {
        window?.show()
        window?.focus()
      })
      .show()
  }
}
const browserPanel = new BrowserPanel(() => window, emit)
const terminals = new TerminalManager({ emit })
let launches: LaunchManager | undefined
const windowViewer = new WindowViewer({
  emit,
  busySessions: async (workspaceId) => {
    const sessions = await requirePrime().call(workspaceId, 'sessions')
    return (Array.isArray(sessions) ? sessions : [])
      .filter((item) => item.is_busy)
      .map((item) => item.id)
  }
})
const jobs = new JobCenter({
  router: () => router(),
  automations: () => automations,
  teams: () => teams,
  launches: () => launches,
  terminals,
  label: (project) =>
    project.kind === 'chat'
      ? t('Личные чаты')
      : project.remote
        ? `${project.remote.name} · ${project.remote.root.split('/').pop() || '/'}`
        : project.path.split(/[\\/]/).pop() || project.path
})
const projectAgentTools = createProjectTools({
  target: (workspaceId) => ({
    workspaceId: projectId.parse(workspaceId),
    root: requirePrime().root(workspaceId),
    key: workspacePath(workspaceId),
    remote: remotes?.byWorkspace(workspaceId)?.connection
  }),
  automations: () => automations,
  launches: () => launches,
  teams: () => teams,
  terminals: () => terminals,
  settings: () => settings,
  files,
  reveal: (workspaceId, view) =>
    emit({ workspaceId, type: 'workbench', payload: { action: 'activate', view } }),
  openSession: (workspaceId, sessionId) =>
    emit({ workspaceId, type: 'workbench', payload: { action: 'openSession', sessionId } }),
  onContextChanged: (workspaceId, context) =>
    emit({
      workspaceId,
      type: 'workbench',
      payload: { action: 'contextChanged', key: workspacePath(workspaceId), context }
    }),
  extensions: [...createGithubReadTools(runGithub), ...createGitHistoryTools(runGithub)]
})
function runGithub(workspaceId: string, request: unknown) {
  const remote = remotes?.byWorkspace(projectId.parse(workspaceId))?.connection
  const transport = remote
    ? `ssh:${remote.profile.user}@${remote.profile.host}:${remote.profile.port}`
    : 'local'
  return githubOperation(
    remote ? remoteRunner(remote) : localRunner(requirePrime().root(workspaceId)),
    request,
    transport
  )
}
const sshAccess = new SshAccess({
  manager: () => {
    if (!remotes) throw new Error('SSH manager is not ready')
    return remotes.manager
  },
  byWorkspace: (workspaceId) => remotes?.byWorkspace(workspaceId)?.connection
})
const agentTools = () => [
  ...workbenchTerminalTools(terminals),
  ...projectAgentTools,
  ...sshProfileTools(sshAccess),
  ...sshUploadTools(sshAccess, (workspaceId) => requirePrime().root(workspaceId)),
  ...createSshExecutionTools((workspaceId, profileId) => sshAccess.resolve(workspaceId, profileId)),
  ...windowViewer.tools()
]
const speech = new SpeechService(
  (status) => emit({ workspaceId: '', type: 'speech', payload: status }),
  async () => {
    const result = await dialog.showOpenDialog(window!, {
      properties: ['openFile'],
      title: t('Стиль голоса Supertonic'),
      filters: [{ name: 'JSON', extensions: ['json'] }]
    })
    return result.canceled ? undefined : result.filePaths[0]
  }
)
const hasBackgroundWork = () =>
  Boolean(
    prime?.isBusy() ||
    remotes?.isBusy() ||
    terminals.list().some((item) => item.status === 'running') ||
    automations?.counts().activeCount ||
    teams?.activeCount()
  )
const shouldKeepRunning = () => hasBackgroundWork() || Boolean(automations?.counts().enabledCount)
async function recoverBackend(): Promise<void> {
  recovering = true
  automations?.stop()
  const previous = [...(prime?.projects.values() || [])]
  prime = undefined
  try {
    while (restarts.filter((time) => Date.now() - time < 60000).length < 3 && !quitting) {
      restarts.push(Date.now())
      await new Promise((resolve) => setTimeout(resolve, 500))
      runtimeError = ''
      await startRuntime(false)
      const recoveredService = prime as PrimeService | undefined
      if (!recoveredService) continue
      for (const project of previous) {
        try {
          const recovered = await recoveredService.open(project.path, project.channels)
          terminals.rebindWorkspace(project.id, recovered.id)
          recovered.kind = project.kind
          if (project.currentSessionId)
            await recoveredService.call(recovered.id, 'currentSession', {
              sessionId: project.currentSessionId
            })
          await recoveredService.call(recovered.id, 'init').catch(() => {})
        } catch (error) {
          runtimeError = (error as Error).message
        }
      }
      emit({ type: 'recovered', payload: [...recoveredService.projects.values()] })
      automations?.start()
      emit({ type: 'runtime', payload: { status: 'ready', version: runtime.version } })
      return
    }
    emit({
      type: 'runtime',
      payload: {
        status: 'failed',
        error:
          runtimeError ||
          t('Не удалось восстановить Prime после трёх попыток. Перезапустите приложение.')
      }
    })
  } finally {
    recovering = false
  }
}
function router(): WorkspaceRouter | undefined {
  return prime && new WorkspaceRouter(prime, () => remotes?.backends() ?? [])
}
function requirePrime(): WorkspaceRouter {
  const current = router()
  if (!current) throw new Error(runtimeError || t('Prime ещё запускается'))
  return current
}
function workspacePath(id: string): string {
  const project = requirePrime().projects.get(projectId.parse(id))
  if (!project) throw new Error(t('Проект не открыт'))
  return project.path
}
function files(id: string) {
  const parsed = projectId.parse(id)
  return projectFiles(
    { root: requirePrime().root(parsed), remote: remotes?.byWorkspace(parsed)?.connection },
    t
  )
}
function localOnly(id: string) {
  if (remotes?.byWorkspace(projectId.parse(id)))
    throw new Error(t('Действие недоступно для удалённого проекта'))
}
function handle(name: string, handler: (...args: any[]) => any): void {
  ipcMain.handle(`prime:${name}`, async (event, ...args) => {
    if (
      !window ||
      event.sender !== window.webContents ||
      event.senderFrame !== window.webContents.mainFrame
    )
      throw new Error(t('Недопустимый источник запроса'))
    return handler(...args)
  })
}
async function startRuntime(notify = true): Promise<void> {
  runtime = new RuntimeSupervisor(
    app.isPackaged
      ? join(process.resourcesPath, 'runtime')
      : join(import.meta.dirname, '../../runtime'),
    emit
  )
  try {
    await runtime.start()
    prime = new PrimeService(runtime.transport!, emit)
    if (notify) emit({ type: 'runtime', payload: { status: 'ready', version: runtime.version } })
  } catch (error) {
    runtime.status = 'failed'
    runtimeError = (error as Error).message
    emit({ type: 'runtime', payload: { status: 'failed', error: runtimeError } })
  }
}
function refreshTrayMenu(): void {
  tray?.setContextMenu(
    Menu.buildFromTemplate([
      {
        label: t('Открыть Prime'),
        click: () => {
          window?.show()
          window?.focus()
        }
      },
      { type: 'separator' },
      {
        label: t('Выйти'),
        click: async () => {
          if (hasBackgroundWork()) {
            const { response } = await dialog.showMessageBox(window!, {
              type: 'question',
              message: t('Остановить задачи и выйти?'),
              buttons: [t('Продолжить работу'), t('Остановить и выйти')],
              defaultId: 0,
              cancelId: 0
            })
            if (response !== 1) return
          }
          quitting = true
          app.quit()
        }
      }
    ])
  )
}
function confirmEditorDiscard(): boolean {
  if (!editorDirty) return true
  const result = dialog.showMessageBoxSync(window!, {
    type: 'question',
    message: t('Есть несохранённые изменения в файлах'),
    buttons: [t('Продолжить редактирование'), t('Закрыть без сохранения')],
    defaultId: 0,
    cancelId: 0
  })
  if (result !== 1) return false
  editorDirty = false
  return true
}
function registerIpc(): void {
  handle('importImages', (input: unknown) =>
    importImages(attachmentStore, input, (bytes) => {
      const image = nativeImage.createFromBuffer(bytes)
      if (image.isEmpty()) throw new Error(t('Не удалось прочитать изображение'))
      const size = image.getSize()
      const scale = Math.min(240 / size.width, 160 / size.height, 1)
      return image
        .resize({
          width: Math.max(1, Math.round(size.width * scale)),
          height: Math.max(1, Math.round(size.height * scale)),
          quality: 'good'
        })
        .toDataURL()
    })
  )
  handle('releaseAttachments', (input: unknown) => {
    for (const id of z.array(z.string().uuid()).max(100).parse(input)) attachmentStore.delete(id)
  })
  handle('launch', (id: string, request: unknown) => {
    if (!launches) throw new Error(t('Prime ещё запускается'))
    const workspaceId = projectId.parse(id)
    return launches.handle(
      {
        workspaceId,
        root: requirePrime().root(workspaceId),
        key: workspacePath(workspaceId),
        remote: remotes?.byWorkspace(workspaceId)?.connection
      },
      request
    )
  })
  handle('browserPick', async () => {
    const captures = await pickElements(browserPanel.contents)
    if (!captures) return null
    return captures.map(({ element, image }) => {
      if (!image) return { element }
      const content = image.toString('base64')
      const attachmentId = randomUUID()
      attachmentStore.set(attachmentId, {
        file_path: 'element.png',
        file_name: 'element.png',
        mime_type: 'image/png',
        content
      })
      return { element, attachmentId, image: `data:image/png;base64,${content}` }
    })
  })
  handle('browserPickCancel', () => cancelPick(browserPanel.contents))
  handle('jobs', (request: unknown) => jobs.handle(request))
  handle('windows', (id: string, request: unknown) =>
    windowViewer.handle(projectId.parse(id), request)
  )
  handle('github', (id: string, request: unknown) => runGithub(projectId.parse(id), request))
  handle('speech', (request: unknown) =>
    speech.handle(
      z
        .discriminatedUnion('action', [
          z.object({ action: z.enum(['status', 'prepare', 'voices', 'import']) }),
          z.object({ action: z.literal('remove'), voice: z.string().max(100) }),
          z.object({
            action: z.literal('speak'),
            text: z.string().max(8000),
            voice: z.string().max(100).optional(),
            speed: z.number().optional()
          })
        ])
        .parse(request)
    )
  )
  handle('browserLogs', (clear?: boolean) => {
    if (clear === true) browserPanel.clearLogs()
    return browserPanel.logs()
  })
  handle('team', (id: string, request: unknown) => {
    if (!teams) throw new Error(t('Планировщик ещё загружается'))
    return teamOperation(teams, workspacePath(id), request)
  })
  handle('automation', (id: string, request: unknown) => {
    const root = workspacePath(id)
    if (!automations) throw new Error(t('Планировщик ещё загружается'))
    return automationOperation(automations, root, request)
  })
  handle('terminal', (id: string, request: unknown) =>
    terminalOperation(
      terminals,
      projectId.parse(id),
      requirePrime().root(id),
      request,
      remotes?.byWorkspace(id)?.connection
    )
  )
  handle('remote', (request: unknown) => {
    if (!remotes) throw new Error(t('Prime ещё запускается'))
    return remoteRequest(remotes.manager, request)
  })
  handle('remoteKey', async () => {
    const result = await dialog.showOpenDialog(window!, {
      properties: ['openFile', 'showHiddenFiles'],
      title: t('Приватный ключ SSH'),
      defaultPath: join(app.getPath('home'), '.ssh')
    })
    return result.canceled ? undefined : result.filePaths[0]
  })
  handle('openRemote', async (profileId: string) => {
    if (!remotes) throw new Error(t('Prime ещё запускается'))
    const remote = await remotes.open(z.string().uuid().parse(profileId))
    const project = requirePrime().projects.get(remote.id)
    if (!project) throw new Error(t('Сервер отключён.'))
    await settings.update({
      activeProject: project.path,
      recent: [
        project.path,
        ...settings.get().recent.filter((item) => item !== project.path)
      ].slice(0, 15)
    })
    return project
  })
  handle('editorDirty', (dirty: boolean) => {
    editorDirty = z.boolean().parse(dirty)
  })
  handle('saveFile', (id: string, path: string, content: string, revision: string) =>
    files(id).save(
      z.string().max(4096).parse(path),
      z
        .string()
        .max(2 * 1024 * 1024)
        .parse(content),
      z
        .string()
        .regex(/^[a-f0-9]{64}$/)
        .parse(revision)
    )
  )
  handle('changes', (id: string) => files(id).changes())
  handle('info', () => ({
    version: app.getVersion(),
    runtime: runtime?.status || 'starting',
    terminalCount: terminals.list().filter((item) => item.status === 'running').length,
    automationCounts: automations?.counts() || { activeCount: 0, enabledCount: 0 },
    runtimeVersion: runtime?.version,
    error: runtimeError,
    packaged: app.isPackaged,
    update: updateStatus
  }))
  handle('openChat', async () => {
    const path = join(app.getPath('userData'), 'Chats')
    await mkdir(path, { recursive: true })
    const workspace = await requirePrime().open(path)
    workspace.kind = 'chat'
    await settings.update({ activeProject: workspace.path })
    return workspace
  })
  handle('projects', () => [...requirePrime().projects.values()])
  handle('openProject', async (path?: string) => {
    if (
      path &&
      !settings.get().recent.includes(path) &&
      !(isRemoteWorkspace(path) && settings.get().activeProject === path) &&
      path !== process.env.PRIME_DESKTOP_TEST_PROJECT
    )
      throw new Error(t('Папку нужно выбрать через диалог'))
    if (!path) {
      const result = await dialog.showOpenDialog(window!, {
        properties: ['openDirectory'],
        title: t('Открыть проект')
      })
      if (result.canceled) return null
      path = result.filePaths[0]
    }
    const target = path
    if (isRemoteWorkspace(target)) {
      const identity = remoteWorkspaceIdentity(target)
      if (!identity || !remotes) throw new Error(t('Сервер отключён.'))
      await remotes.open(identity.profileId)
    }
    projectWait?.abort()
    const wait = new AbortController()
    projectWait = wait
    let project
    try {
      project = await openWhenUnlocked({
        open: () => requirePrime().open(target, settings.get().projectChannels?.[target] || []),
        signal: wait.signal,
        onWait: () =>
          emit({ workspaceId: '', type: 'projectLock', payload: { path: target, waiting: true } }),
        onDone: () =>
          emit({ workspaceId: '', type: 'projectLock', payload: { path: target, waiting: false } })
      })
    } catch (error) {
      if ((error as { cancelled?: boolean }).cancelled) return null
      if (isProjectLocked(error))
        throw new Error(
          t('Проект открыт в Prime CLI. Закройте CLI в этой папке и откройте проект снова.')
        )
      throw error
    } finally {
      if (projectWait === wait) projectWait = undefined
    }
    const preferences = settings.get()
    await settings.update({
      activeProject: project.path,
      recent: [project.path, ...preferences.recent.filter((item) => item !== project.path)].slice(
        0,
        15
      )
    })
    return project
  })
  handle('cancelProjectWait', () => projectWait?.abort())
  handle('closeProject', async (id: string) => {
    if (requirePrime().isBusy()) throw new Error(t('Сначала завершите активные задачи'))
    if (terminals.list(id).some((item) => item.status === 'running'))
      throw new Error(t('Сначала закройте терминалы проекта'))
    await Promise.all(terminals.list(id).map((item) => terminals.close(item.id, id)))
    const remote = remotes?.byWorkspace(projectId.parse(id))
    if (remote) await remotes!.manager.disconnect(remote.profile.id)
    else await requirePrime().close(id)
    windowViewer.forget(id)
    const remaining = [...requirePrime().projects.values()]
    await settings.update({ activeProject: remaining[0]?.path || '' })
  })
  handle('call', async (id: string, operation: Operation, args: any = {}) => {
    projectId.parse(id)
    z.string().max(40).parse(operation)
    if (
      !args ||
      typeof args !== 'object' ||
      Array.isArray(args) ||
      JSON.stringify(args).length > 30 * 1024 * 1024
    )
      throw new Error(t('Некорректные параметры запроса'))
    if (operation === 'send') {
      z.string().uuid().parse(args.runId)
      z.string()
        .min(1)
        .max(2 * 1024 * 1024)
        .parse(args.prompt)
      args.attachments = (args.attachmentIds || []).map((key: string) => {
        const attachment = attachmentStore.get(key)
        if (!attachment) throw new Error(t('Вложение больше недоступно'))
        return attachment
      })
    }
    const result = await requirePrime().call(id, operation, args)
    if (operation === 'currentSession')
      await settings.update({
        activeProject: workspacePath(id),
        activeSessions: {
          ...settings.get().activeSessions,
          [workspacePath(id)]: args.sessionId
        }
      })
    return result
  })
  handle('files', (id: string, path = '') => files(id).list(z.string().max(4096).parse(path)))
  handle('readFile', (id: string, path: string) => files(id).read(z.string().max(4096).parse(path)))
  handle('diff', async (id: string, sessionId: string, path: string, source: 'session' | 'git') => {
    const history =
      source === 'session' ? await requirePrime().call(id, 'history', { sessionId }) : []
    z.enum(['session', 'git']).parse(source)
    return files(id).diff(z.string().max(4096).parse(path), source, history || [])
  })
  handle('attachments', async () => {
    const result = await dialog.showOpenDialog(window!, {
      properties: ['openFile', 'multiSelections']
    })
    if (result.canceled) return []
    const output: any[] = []
    const mimeTypes: Record<string, string> = {
      '.png': 'image/png',
      '.jpg': 'image/jpeg',
      '.jpeg': 'image/jpeg',
      '.webp': 'image/webp',
      '.pdf': 'application/pdf',
      '.gif': 'image/gif'
    }
    for (const path of result.filePaths) {
      const info = await stat(path)
      if (info.size > 20 * 1024 * 1024) throw new Error(t('Вложение больше 20 MiB'))
      const id = randomUUID()
      attachmentStore.set(id, {
        file_path: path,
        file_name: basename(path),
        mime_type: mimeTypes[extname(path).toLowerCase()] || 'text/plain',
        content: (await readFile(path)).toString('base64')
      })
      output.push({ id, name: basename(path), size: info.size })
    }
    return output
  })
  handle('preferences', async (patch?: any) => {
    if (!patch) return settings.get()
    const valid = z
      .object({
        theme: z.enum(['dark', 'light', 'system']).optional(),
        language: z.enum(['ru', 'en']).optional(),
        density: z.enum(['comfortable', 'compact']).optional(),
        reduceMotion: z.boolean().optional(),
        sidebarVisible: z.boolean().optional(),
        speechVoice: z.string().max(100).optional(),
        speechSpeed: z.number().min(0.7).max(1.6).optional(),
        speechAutoSend: z.boolean().optional(),
        inspectorVisible: z.boolean().optional(),
        keybindings: z.record(z.string().max(40), z.string().max(80)).optional(),
        agentContexts: z
          .record(
            z.string().max(4096),
            z.object({
              enabled: z.boolean(),
              goal: z.string().max(4000),
              constraints: z.string().max(4000),
              criteria: z.string().max(4000),
              sources: z
                .array(
                  z.object({
                    id: z.string().max(8192),
                    kind: z.enum(['file', 'selection']),
                    path: z.string().min(1).max(4096),
                    content: z.string().max(12000).optional(),
                    startLine: z.number().int().positive().optional(),
                    endLine: z.number().int().positive().optional()
                  })
                )
                .max(8)
            })
          )
          .optional(),
        pinned: z.array(z.string()).max(500).optional(),
        favoriteModels: z
          .array(
            z.object({ provider: z.string().min(1).max(512), model: z.string().min(1).max(1024) })
          )
          .max(100)
          .optional(),
        recentModels: z
          .array(
            z.object({ provider: z.string().min(1).max(512), model: z.string().min(1).max(1024) })
          )
          .max(8)
          .optional(),
        reasoningBySession: z
          .record(
            z.string().min(1).max(200),
            z.enum(['off', 'low', 'medium', 'high', 'xhigh', 'max', 'default'])
          )
          .optional(),
        drafts: z.record(z.string(), z.string().max(200000)).optional(),
        sidebarWidth: z.number().min(200).max(500).optional(),
        inspectorWidth: z.number().finite().min(280).optional(),
        projectChannels: z
          .record(z.string(), z.array(z.string().min(1).max(200)).max(100))
          .optional()
      })
      .parse(patch)
    const updated = await settings.update(valid)
    if (valid.language) refreshTrayMenu()
    if (valid.theme) syncTitlebar()
    return updated
  })
  handle('windowDimmed', (value: unknown) => {
    windowDimmed = value === true
    syncTitlebar()
  })
  handle('openExternal', (url: string) =>
    shell.openExternal(validateExternalUrl(z.string().max(8192).parse(url)))
  )
  handle('createEntry', (id: string, path: string, kind: string) =>
    files(id).create(
      z.string().min(1).max(4096).parse(path),
      z.enum(['file', 'folder']).parse(kind)
    )
  )
  handle('trashEntry', (id: string, path: string) =>
    files(id).remove(z.string().min(1).max(4096).parse(path), (absolute) =>
      shell.trashItem(absolute)
    )
  )
  handle('openWith', async (id: string, path: string, target: string) => {
    const relative = z.string().max(4096).parse(path)
    if (z.enum(['default', 'vscode']).parse(target) === 'vscode') {
      await shell.openExternal(await files(id).editorUrl(relative))
      return
    }
    localOnly(id)
    const root = requirePrime().root(id)
    const failure = await shell.openPath(await resolveProjectPath(root, relative))
    if (failure) throw new Error(failure)
  })
  const bounds = z.object({
    x: z.number().finite(),
    y: z.number().finite(),
    width: z.number().finite().min(0),
    height: z.number().finite().min(0)
  })
  handle('browserShow', (value: unknown) => browserPanel.show(bounds.parse(value)))
  handle('browserHide', () => browserPanel.hide())
  handle('browserTab', (action: string, id?: string, url?: string) => {
    const command = z.enum(['new', 'select', 'close']).parse(action)
    if (command === 'new')
      return browserPanel.newTab(url ? z.string().max(8192).parse(url) : undefined)
    const tab = z.string().uuid().parse(id)
    return command === 'select' ? browserPanel.selectTab(tab) : browserPanel.closeTab(tab)
  })
  handle('browserState', () => browserPanel.state())
  handle('browserNavigate', (url: string) => browserPanel.navigate(z.string().max(8192).parse(url)))
  handle('browserCommand', (command: string) => {
    const action = z.enum(['back', 'forward', 'reload']).parse(command)
    browserPanel[action]()
    return browserPanel.state()
  })
  handle('browserOpenExternal', async () => {
    const url = browserPanel.state().url
    if (url) await openInSystemBrowser(url)
  })
  handle('reveal', async (id: string, path = '') => {
    localOnly(id)
    const resolved = await resolveProjectPath(requirePrime().root(projectId.parse(id)), path)
    if (path) shell.showItemInFolder(resolved)
    else await shell.openPath(resolved)
  })
  handle('update', async (action: string) => {
    z.enum(['check', 'install']).parse(action)
    if (!app.isPackaged)
      return { state: 'unavailable', message: t('Обновления доступны в установленном приложении') }
    updateController ||= (async () => {
      const { autoUpdater } = (await import('electron-updater')).default
      autoUpdater.allowDowngrade = false
      autoUpdater.allowPrerelease = false
      return new UpdateController(
        autoUpdater,
        () => discoverDesktopRelease(app.getVersion()),
        (status) => {
          updateStatus = status
          emit({ type: 'update', payload: status })
        },
        async () => {
          if (hasBackgroundWork()) throw new Error(t('Сначала завершите активные задачи'))
          if (editorDirty) throw new Error(t('Сохраните изменения в файлах перед перезапуском'))
          await automations?.dispose()
        }
      )
    })()
    const controller = await updateController
    try {
      return await (action === 'install' ? controller.install() : controller.check())
    } catch (error) {
      if (action === 'install') automations?.start()
      throw error
    }
  })
}
let windowDimmed = false
function syncTitlebar(): void {
  applyWindowChrome(window, settings.get().theme, windowDimmed)
}
function createWindow(): void {
  applyWindowChrome(undefined, settings.get().theme)
  const chrome = windowChromeColors()
  window = new BrowserWindow({
    width: 1440,
    height: 940,
    minWidth: 420,
    minHeight: 420,
    show: false,
    backgroundColor: chrome.color,
    accentColor: chrome.color,
    icon: app.isPackaged
      ? join(process.resourcesPath, 'icon.png')
      : join(import.meta.dirname, '../../build/icon.png'),
    title: 'Prime Desktop',
    titleBarStyle: 'hidden',
    titleBarOverlay: chrome,
    autoHideMenuBar: true,
    webPreferences: {
      preload: join(import.meta.dirname, '../preload/index.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    }
  })
  syncTitlebar()
  window.once('ready-to-show', () => window?.show())
  window.webContents.on('will-prevent-unload', (event) => {
    if (confirmEditorDiscard()) event.preventDefault()
  })
  window.webContents.setWindowOpenHandler(({ url }) => {
    try {
      void shell.openExternal(validateExternalUrl(url))
    } catch {}
    return { action: 'deny' }
  })
  window.webContents.on('will-navigate', (event) => event.preventDefault())
  window.on('close', (event) => {
    if (!quitting && shouldKeepRunning()) {
      event.preventDefault()
      window?.hide()
      new Notification({
        title: 'Prime Desktop',
        body: automations?.counts().enabledCount
          ? t('Планировщик работает в трее. Для выхода используйте меню значка.')
          : t('Задачи продолжают работать. Приложение доступно в трее.')
      }).show()
    }
  })
  if (process.env.ELECTRON_RENDERER_URL) void window.loadURL(process.env.ELECTRON_RENDERER_URL)
  else void window.loadFile(join(import.meta.dirname, '../renderer/index.html'))
}
if (!app.requestSingleInstanceLock()) app.quit()
else {
  app.on('second-instance', () => {
    window?.show()
    window?.focus()
  })
  app.whenReady().then(async () => {
    settings = new SettingsStore(join(app.getPath('userData'), 'preferences.json'))
    await settings.load()
    const allowedPermissions = new Set(['media', 'clipboard-sanitized-write'])
    session.defaultSession.setPermissionRequestHandler((webContents, permission, callback) =>
      callback(allowedPermissions.has(permission) && webContents === window?.webContents)
    )
    registerIpc()
    globalShortcut.register('Control+Alt+Escape', () => windowViewer.revokeAll())
    createWindow()
    nativeTheme.on('updated', syncTitlebar)
    const image = nativeImage
      .createFromPath(
        app.isPackaged
          ? join(process.resourcesPath, 'icon.png')
          : join(import.meta.dirname, '../../build/icon.png')
      )
      .resize({ width: 24, height: 24 })
    tray = new Tray(image)
    tray.setToolTip('Prime Desktop')
    tray.on('double-click', () => window?.show())
    refreshTrayMenu()
    try {
      browserMcp = await startBrowserMcpServer(browserPanel, {
        tools: agentTools,
        resolveWorkspace: (root) =>
          [...(prime?.projects.values() || [])].find(
            (project) =>
              project.path.replaceAll('\\', '/').toLowerCase() ===
              root.replaceAll('\\', '/').toLowerCase()
          )?.id
      })
      process.env.PRIME_DESKTOP_BROWSER_URL = browserMcp.url
      process.env.PRIME_DESKTOP_BROWSER_TOKEN = browserMcp.token
    } catch (error) {
      console.error('Built-in browser tools are unavailable', error)
    }
    remotes = new RemoteWorkspaces({
      path: join(app.getPath('userData'), 'remote-profiles.json'),
      credentials: createCredentialVault(join(app.getPath('userData'), 'remote-credentials.json')),
      workspaceDirectory: join(app.getPath('userData'), 'SSH'),
      service: () => {
        if (!prime) throw new Error(t('Prime ещё запускается'))
        return prime
      },
      emit,
      channels: (key) => settings.get().projectChannels?.[key] || [],
      onDetached: async (workspaceId) => {
        windowViewer.forget(workspaceId)
        await terminals.closeWorkspace(workspaceId)
      }
    })
    await remotes.manager.load()
    await startRuntime()
    automations = new AutomationManager({
      path: join(app.getPath('userData'), 'automations.json'),
      emit,
      adapter: createAgentAdapter({
        service: requirePrime,
        channels: (root) => settings.get().projectChannels?.[root] || [],
        filesFingerprint: (root) => {
          if (!isRemoteWorkspace(root)) return projectChanges.fingerprint(root)
          const remote = remotes?.backends().find((entry) => entry.key === root)
          if (!remote) return Promise.reject(new Error(t('Сервер отключён.')))
          return remoteFingerprint(remote.connection)
        },
        onProject: () => emit({ type: 'projectsChanged', payload: {} })
      })
    })
    await automations.load()
    if (prime) automations.start()
    teams = new TeamManager({
      path: join(app.getPath('userData'), 'teams.json'),
      emit,
      adapter: createAgentAdapter({
        service: requirePrime,
        channels: (root) => settings.get().projectChannels?.[root] || [],
        onProject: () => emit({ type: 'projectsChanged', payload: {} })
      })
    })
    await teams.load()
    teams.start()
    launches = new LaunchManager({
      path: join(app.getPath('userData'), 'launch.json'),
      terminals,
      emit,
      onUrl: (url) => void browserPanel.navigate(url).catch(() => {})
    })
    await launches.load()
  })
  app.on('window-all-closed', () => {
    if (!shouldKeepRunning()) app.quit()
  })
  app.on('before-quit', (event) => {
    if (cleanupDone) return
    event.preventDefault()
    if (cleanupRunning) return
    if (!confirmEditorDiscard()) {
      quitting = false
      return
    }
    quitting = true
    cleanupRunning = true
    globalShortcut.unregister('Control+Alt+Escape')
    windowViewer.dispose()
    void (async () => {
      await automations?.dispose()
      await teams?.dispose()
      await speech.dispose()
      projectChanges.dispose()
      await terminals.dispose()
      await remotes?.shutdown()
      await prime?.dispose()
      await runtime?.stop()
      await browserMcp?.close()
      browserPanel.destroy()
      tray?.destroy()
      cleanupDone = true
      app.quit()
    })()
  })
}
