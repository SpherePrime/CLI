const { app, BrowserWindow } = require('electron')
app.whenReady().then(() => {
  const window = new BrowserWindow({
    x: -4000,
    y: 0,
    width: 1000,
    height: 760,
    show: true,
    skipTaskbar: true,
    focusable: false,
    webPreferences: { backgroundThrottling: false }
  })
  void window.loadURL(process.env.PRIME_HISTORY_TEST_URL)
})
app.on('window-all-closed', () => app.quit())
