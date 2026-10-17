import { translate } from '../shared/localization'
import {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
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
import { listFiles, readProjectFile, fileDiff } from './files/browser'
import { resolveProjectPath } from './files/paths'
import { gitChanges } from './files/git'
import { validateExternalUrl } from './ipc/security'
import type { Operation } from '../shared/contracts'

let window: BrowserWindow | null = null
let tray: Tray | null = null
let quitting = false
let cleanupDone = false
let runtime: RuntimeSupervisor
let prime: PrimeService | undefined
let settings: SettingsStore
let runtimeError = ''
let updateStatus: any = { state: 'idle' }
let recovering = false
const t = (source: string) => translate(settings?.get().language, source)
const restarts: number[] = []
const attachmentStore = new Map<string, any>()
const projectId = z.string().uuid()
const emit = (event: any) => {
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
async function recoverBackend(): Promise<void> {
  recovering = true
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
function requirePrime(): PrimeService {
  if (!prime) throw new Error(runtimeError || t('Prime ещё запускается'))
  return prime
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
          if (prime?.isBusy()) {
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
function registerIpc(): void {
  handle('changes', (id: string) => gitChanges(requirePrime().root(projectId.parse(id))))
  handle('info', () => ({
    version: app.getVersion(),
    runtime: runtime?.status || 'starting',
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
    const project = await requirePrime().open(path, settings.get().projectChannels?.[path] || [])
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
  handle('closeProject', async (id: string) => {
    if (requirePrime().isBusy()) throw new Error(t('Сначала завершите активные задачи'))
    await requirePrime().close(projectId.parse(id))
    await settings.update({ activeProject: '' })
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
        activeProject: requirePrime().root(id),
        activeSessions: {
          ...settings.get().activeSessions,
          [requirePrime().root(id)]: args.sessionId
        }
      })
    return result
  })
  handle('files', (id: string, path = '') =>
    listFiles(requirePrime().root(projectId.parse(id)), z.string().max(4096).parse(path))
  )
  handle('readFile', (id: string, path: string) =>
    readProjectFile(requirePrime().root(projectId.parse(id)), z.string().max(4096).parse(path))
  )
  handle('diff', async (id: string, sessionId: string, path: string, source: 'session' | 'git') => {
    const history =
      source === 'session' ? await requirePrime().call(id, 'history', { sessionId }) : []
    z.enum(['session', 'git']).parse(source)
    return fileDiff(requirePrime().root(projectId.parse(id)), path, source, history || [])
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
        pinned: z.array(z.string()).max(500).optional(),
        drafts: z.record(z.string(), z.string().max(200000)).optional(),
        sidebarWidth: z.number().min(200).max(500).optional(),
        inspectorWidth: z.number().min(280).max(700).optional(),
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
  handle('openExternal', (url: string) =>
    shell.openExternal(validateExternalUrl(z.string().max(8192).parse(url)))
  )
  handle('reveal', async (id: string, path = '') => {
    const resolved = await resolveProjectPath(requirePrime().root(projectId.parse(id)), path)
    if (path) shell.showItemInFolder(resolved)
    else await shell.openPath(resolved)
  })
  handle('update', async (action: string) => {
    if (!app.isPackaged)
      return { state: 'unavailable', message: t('Обновления доступны в установленном приложении') }
    if (action === 'install' && requirePrime().isBusy())
      throw new Error(t('Сначала завершите активные задачи'))
    try {
      const { autoUpdater } = await import('electron-updater')
      if (action === 'install') {
        quitting = true
        autoUpdater.quitAndInstall()
        return { state: 'installing' }
      }
      autoUpdater.on('update-downloaded', () => {
        updateStatus = { state: 'downloaded' }
        emit({ type: 'update', payload: updateStatus })
      })
      const result = await autoUpdater.checkForUpdates()
      updateStatus = {
        state: result?.isUpdateAvailable ? 'available' : 'current',
        version: result?.updateInfo.version
      }
      return updateStatus
    } catch {
      return {
        state: 'unavailable',
        message: t('Канал обновлений ещё не настроен для этой сборки')
      }
    }
  })
}
function syncTitlebar(): void {
  const theme = settings.get().theme
  const dark = theme === 'system' ? nativeTheme.shouldUseDarkColors : theme !== 'light'
  window?.setTitleBarOverlay({
    color: dark ? '#131313' : '#efede7',
    symbolColor: dark ? '#f1efe9' : '#17110d',
    height: 40
  })
}
function createWindow(): void {
  window = new BrowserWindow({
    width: 1440,
    height: 940,
    minWidth: 800,
    minHeight: 600,
    backgroundColor: '#101010',
    icon: app.isPackaged
      ? join(process.resourcesPath, 'icon.png')
      : join(import.meta.dirname, '../../build/icon.png'),
    title: 'Prime Desktop',
    titleBarStyle: 'hidden',
    titleBarOverlay: { color: '#131313', symbolColor: '#f1efe9', height: 40 },
    autoHideMenuBar: true,
    webPreferences: {
      preload: join(import.meta.dirname, '../preload/index.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    }
  })
  syncTitlebar()
  window.webContents.setWindowOpenHandler(({ url }) => {
    try {
      void shell.openExternal(validateExternalUrl(url))
    } catch {}
    return { action: 'deny' }
  })
  window.webContents.on('will-navigate', (event) => event.preventDefault())
  window.on('close', (event) => {
    if (!quitting && prime?.isBusy()) {
      event.preventDefault()
      window?.hide()
      new Notification({
        title: 'Prime Desktop',
        body: t('Задачи продолжают работать. Приложение доступно в трее.')
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
    session.defaultSession.setPermissionRequestHandler((_webContents, permission, callback) =>
      callback(permission === 'media')
    )
    registerIpc()
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
    await startRuntime()
  })
  app.on('window-all-closed', () => {
    if (!prime?.isBusy()) app.quit()
  })
  app.on('before-quit', (event) => {
    if (cleanupDone) return
    event.preventDefault()
    quitting = true
    void (async () => {
      await prime?.dispose()
      await runtime?.stop()
      tray?.destroy()
      cleanupDone = true
      app.quit()
    })()
  })
}
