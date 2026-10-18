const { app, BrowserWindow } = require('electron')
app.whenReady().then(() => {
  const window = new BrowserWindow({ width: 1000, height: 690, frame: false, show: false, backgroundColor: '#101010', webPreferences: { backgroundThrottling: false } })
  void window.loadURL(process.env.PRIME_SETUP_TEST_URL)
})
app.on('window-all-closed', () => app.quit())
