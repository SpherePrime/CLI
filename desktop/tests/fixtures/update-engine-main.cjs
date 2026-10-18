const { app, BrowserWindow } = require('electron')
app.whenReady().then(async () => {
  const { autoUpdater } = (await import('electron-updater')).default
  autoUpdater.forceDevUpdateConfig = true
  autoUpdater.autoDownload = false
  autoUpdater.autoInstallOnAppQuit = false
  autoUpdater.disableDifferentialDownload = true
  autoUpdater.updateConfigPath = process.env.PRIME_UPDATE_CONFIG
  autoUpdater.setFeedURL({ provider: 'generic', url: process.env.PRIME_UPDATE_URL })
  const states = []
  autoUpdater.on('download-progress', progress => states.push({ state: 'downloading', percent: progress.percent }))
  autoUpdater.on('update-downloaded', info => states.push({ state: 'downloaded', version: info.version }))
  autoUpdater.on('error', error => states.push({ state: 'error', message: error.message }))
  global.updateEngine = {
    states,
    run: async () => { try { await autoUpdater.checkForUpdates(); await autoUpdater.downloadUpdate(); return { ok: true, states } } catch (error) { return { ok: false, message: error.message, states } } }
  }
  const window = new BrowserWindow({ show: false })
  await window.loadURL('data:text/html,<p>Update engine ready</p>')
})
app.on('window-all-closed', () => app.quit())
