import { contextBridge, ipcRenderer } from 'electron'
import type { DesktopApi } from '../shared/contracts'
const api: DesktopApi = {
  changes: (id) => ipcRenderer.invoke('prime:changes', id),
  info: () => ipcRenderer.invoke('prime:info'),
  projects: () => ipcRenderer.invoke('prime:projects'),
  openProject: (path) => ipcRenderer.invoke('prime:openProject', path),
  closeProject: (id) => ipcRenderer.invoke('prime:closeProject', id),
  call: (id, operation, args) => ipcRenderer.invoke('prime:call', id, operation, args),
  files: (id, path) => ipcRenderer.invoke('prime:files', id, path),
  readFile: (id, path) => ipcRenderer.invoke('prime:readFile', id, path),
  diff: (id, session, path, source) => ipcRenderer.invoke('prime:diff', id, session, path, source),
  attachments: () => ipcRenderer.invoke('prime:attachments'),
  preferences: (patch) => ipcRenderer.invoke('prime:preferences', patch),
  openExternal: (url) => ipcRenderer.invoke('prime:openExternal', url),
  reveal: (id, path) => ipcRenderer.invoke('prime:reveal', id, path),
  update: (action) => ipcRenderer.invoke('prime:update', action),
  onEvent: (listener) => {
    const handler = (_event: Electron.IpcRendererEvent, value: any) => listener(value)
    ipcRenderer.on('prime:event', handler)
    return () => ipcRenderer.removeListener('prime:event', handler)
  }
}
contextBridge.exposeInMainWorld('prime', api)
