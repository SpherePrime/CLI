const { app, BrowserWindow, ipcMain } = require('electron')
const { join } = require('node:path')
const { TerminalManager } = require(process.env.PRIME_TERMINAL_MANAGER)
let window
const writes = []
const manager = new TerminalManager({ emit: (event) => {
  if (window && !window.isDestroyed()) window.webContents.send('terminal-event', event)
} })
ipcMain.handle('terminal-fixture-writes', () => writes)
ipcMain.handle('terminal-fixture', (_event, request) => {
  const workspaceId = 'fixture'
  switch (request.action) {
    case 'list': return manager.list(workspaceId)
    case 'create': return manager.create(workspaceId, process.cwd(), request.options)
    case 'read': {
      const snapshot = manager.read(request.id, workspaceId)
      return { ...snapshot, output: snapshot.output + '\x1b[c' }
    }
    case 'write': writes.push(request.data); return manager.write(request.id, request.data, 'user', workspaceId)
    case 'resize': return manager.resize(request.id, request.cols, request.rows, workspaceId)
    case 'owner': return manager.setOwner(request.id, request.owner, workspaceId)
    case 'close': return manager.close(request.id, workspaceId)
    default: throw new Error('Unknown terminal action')
  }
})
app.whenReady().then(() => {
  window = new BrowserWindow({
    x: -4000, y: 0, width: 1060, height: 760, show: true, skipTaskbar: true, focusable: false,
    webPreferences: { preload: join(__dirname, 'terminal-ui-preload.cjs'), backgroundThrottling: false }
  })
  void window.loadURL(process.env.PRIME_TERMINAL_UI_URL)
})
let quitting = false
app.on('before-quit', (event) => {
  if (quitting) return
  event.preventDefault()
  quitting = true
  void manager.dispose().finally(() => app.exit(0))
})
app.on('window-all-closed', () => app.quit())
