const { contextBridge, ipcRenderer } = require('electron')
contextBridge.exposeInMainWorld('terminalFixture', {
  api: (request) => ipcRenderer.invoke('terminal-fixture', request),
  writes: () => ipcRenderer.invoke('terminal-fixture-writes'),
  subscribe: (listener) => {
    const handler = (_event, payload) => listener(payload)
    ipcRenderer.on('terminal-event', handler)
    return () => ipcRenderer.removeListener('terminal-event', handler)
  }
})
