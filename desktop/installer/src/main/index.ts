import { app, BrowserWindow, dialog, ipcMain } from 'electron'
import { join } from 'node:path'
import { spawn, execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { InstallerController } from './controller'
import { payloadOperations, readPayload } from './payload'

let window: BrowserWindow
let controller: InstallerController
async function defaultDirectory() {
  try {
    const { stdout } = await promisify(execFile)(
      'reg',
      [
        'query',
        'HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall',
        '/s',
        '/f',
        'Prime Desktop',
        '/d',
        '/e'
      ],
      { windowsHide: true }
    )
    const key = stdout.split(/\r?\n/).find((line) => line.startsWith('HKEY_CURRENT_USER\\'))
    if (key) {
      const result = await promisify(execFile)(
        'reg',
        ['query', key.trim(), '/v', 'InstallLocation'],
        { windowsHide: true }
      )
      const location = /InstallLocation\s+REG_SZ\s+(.+)/.exec(result.stdout)?.[1]?.trim()
      if (location) return location
    }
  } catch {}
  return join(process.env.LOCALAPPDATA || app.getPath('home'), 'Programs', 'Prime Desktop')
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
      const payload = await readPayload(root)
      controller = new InstallerController(payloadOperations(root, payload), (status) =>
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
        icon: app.isPackaged
          ? join(process.resourcesPath, 'icon.png')
          : join(import.meta.dirname, '../../../build/icon.png'),
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
        version: payload.version,
        directory: await defaultDirectory(),
        installedSize: payload.installedSize
      }))
      handle('directory', async (current) => {
        if (controller.active) return null
        const result = await dialog.showOpenDialog(window, {
          title: 'Папка для Prime Desktop',
          defaultPath: typeof current === 'string' ? current : await defaultDirectory(),
          properties: ['openDirectory', 'createDirectory']
        })
        return result.canceled ? null : result.filePaths[0]
      })
      handle('install', (directory) => {
        if (typeof directory !== 'string' || directory.length > 240)
          throw new Error('Неверный путь установки')
        return controller.install(directory)
      })
      handle('open', () => {
        const status = controller.snapshot()
        if (status.phase !== 'complete' || !status.directory)
          throw new Error('Установка не завершена')
        const child = spawn(join(status.directory, 'PrimeDesktop.exe'), [], {
          detached: true,
          windowsHide: true,
          stdio: 'ignore'
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
