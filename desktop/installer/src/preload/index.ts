import { contextBridge, ipcRenderer } from 'electron'
import type { SetupApi } from '../shared/contracts'

const api: SetupApi = {
  info: () => ipcRenderer.invoke('setup:info'),
  chooseDirectory: (directory) => ipcRenderer.invoke('setup:directory', directory),
  install: (directory) => ipcRenderer.invoke('setup:install', directory),
  open: () => ipcRenderer.invoke('setup:open'),
  close: () => ipcRenderer.invoke('setup:close'),
  onProgress: (listener) => {
    const handler = (_event: unknown, status: any) => listener(status)
    ipcRenderer.on('setup:progress', handler)
    return () => ipcRenderer.removeListener('setup:progress', handler)
  }
}
contextBridge.exposeInMainWorld('setup', api)
