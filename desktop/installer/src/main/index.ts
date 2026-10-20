import { app, BrowserWindow, dialog, ipcMain } from 'electron'
import { join } from 'node:path'
import { spawn } from 'node:child_process'
import { InstallerController } from './controller'
import { createInstaller } from './payload'
import type { PlatformInstaller } from './platform'

const bundleEnvironment = ['APPIMAGE', 'APPDIR', 'OWD', 'ARGV0', 'APPIMAGE_EXTRACT_AND_RUN']
let window: BrowserWindow
let controller: InstallerController
let installer: PlatformInstaller

function launchEnvironment(extra: NodeJS.ProcessEnv = {}) {
  const env = { ...process.env }
  for (const name of bundleEnvironment) delete env[name]
  return { ...env, ...extra }
}
if (!app.requestSingleInstanceLock()) app.quit()
else {
  app.on('second-instance', () => {
    window?.show()
    window?.focus()
  })
  app
    .whenReady()
    .then(async () => {
      const root = app.isPackaged
        ? join(process.resourcesPath, 'payload')
        : join(import.meta.dirname, '../../payload')
      const icon = app.isPackaged
        ? join(process.resourcesPath, 'icon.png')
        : join(import.meta.dirname, '../../../build/icon.png')
      installer = await createInstaller(root, icon)
      controller = new InstallerController(installer.operations, (status) =>
        window?.webContents.send('setup:progress', status)
      )
      window = new BrowserWindow({
        width: 1000,
        height: 690,
        minWidth: 920,
        minHeight: 650,
        resizable: false,
        frame: false,
        backgroundColor: '#101010',
        title: 'Prime Desktop',
        icon,
        webPreferences: {
          preload: join(import.meta.dirname, '../preload/index.cjs'),
          contextIsolation: true,
          nodeIntegration: false,
          sandbox: true
        }
      })
      const handle = (name: string, action: (...args: any[]) => unknown) =>
        ipcMain.handle(`setup:${name}`, (event, ...args) => {
          if (
            event.sender !== window.webContents ||
            event.senderFrame !== window.webContents.mainFrame
          )
            throw new Error('Недопустимый источник запроса')
          return action(...args)
        })
      handle('info', async () => ({
        version: installer.version,
        directory: await installer.defaultDirectory(),
        installedSize: installer.installedSize,
        method: await installer.method()
      }))
      handle('directory', async (current) => {
        if (controller.active || (await installer.method()).directoryLocked) return null
        const result = await dialog.showOpenDialog(window, {
          title: 'Папка для Prime Desktop',
          defaultPath: typeof current === 'string' ? current : await installer.defaultDirectory(),
          properties: ['openDirectory', 'createDirectory']
        })
        return result.canceled ? null : result.filePaths[0]
      })
      handle('install', async (directory) => {
        if (typeof directory !== 'string' || directory.length > 240)
          throw new Error('Неверный путь установки')
        if ((await installer.method()).directoryLocked)
          directory = await installer.defaultDirectory()
        return controller.install(directory)
      })
      handle('open', async () => {
        const status = controller.snapshot()
        if (status.phase !== 'complete' || !status.directory)
          throw new Error('Установка не завершена')
        const command = await installer.open(status.directory)
        const child = spawn(command.binary, command.args, {
          detached: true,
          windowsHide: true,
          stdio: 'ignore',
          env: launchEnvironment(command.env)
        })
        child.once('error', () =>
          dialog.showErrorBox('Prime Desktop', 'Не удалось открыть приложение')
        )
        child.once('spawn', () => {
          child.unref()
          app.quit()
        })
      })
      handle('close', () => {
        if (!controller.active) app.quit()
      })
      window.on('close', (event) => {
        if (controller.active) event.preventDefault()
      })
      window.webContents.on('will-navigate', (event) => event.preventDefault())
      window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
      if (process.env.ELECTRON_RENDERER_URL) await window.loadURL(process.env.ELECTRON_RENDERER_URL)
      else await window.loadFile(join(import.meta.dirname, '../renderer/index.html'))
    })
    .catch((error) => {
      dialog.showErrorBox('Prime Desktop', error.message)
      app.quit()
    })
  app.on('window-all-closed', () => app.quit())
}
