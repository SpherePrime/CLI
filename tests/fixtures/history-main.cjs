const { app, BrowserWindow } = require('electron')
app.whenReady().then(() => {
  const window = new BrowserWindow({ width: 1100, height: 800, show: false })
  void window.loadURL(process.env.PRIME_HISTORY_TEST_URL)
})
app.on('window-all-closed', () => app.quit())
