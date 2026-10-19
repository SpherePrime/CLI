import { contextBridge, ipcRenderer } from 'electron'
import type { DesktopApi } from '../shared/contracts'
const api: DesktopApi = {
  importImages: images => ipcRenderer.invoke('prime:importImages', images),
  releaseAttachments: ids => ipcRenderer.invoke('prime:releaseAttachments', ids),
  automation: (id, request) => ipcRenderer.invoke('prime:automation', id, request),
  team: (id, request) => ipcRenderer.invoke('prime:team', id, request),
  launch: (id, request) => ipcRenderer.invoke('prime:launch', id, request),
  browserLogs: (clear) => ipcRenderer.invoke('prime:browserLogs', clear),
  speech: (request) => ipcRenderer.invoke('prime:speech', request),
  github: (id, request) => ipcRenderer.invoke('prime:github', id, request),
  windows: (id, request) => ipcRenderer.invoke('prime:windows', id, request),
  jobs: (request) => ipcRenderer.invoke('prime:jobs', request),
  browserPick: () => ipcRenderer.invoke('prime:browserPick'),
  browserPickCancel: () => ipcRenderer.invoke('prime:browserPickCancel'),
  terminal: (id, request) => ipcRenderer.invoke('prime:terminal', id, request),
  changes: (id) => ipcRenderer.invoke('prime:changes', id),
  info: () => ipcRenderer.invoke('prime:info'),
  projects: () => ipcRenderer.invoke('prime:projects'),
  openChat: () => ipcRenderer.invoke('prime:openChat'),
  openProject: (path) => ipcRenderer.invoke('prime:openProject', path),
  cancelProjectWait: () => ipcRenderer.invoke('prime:cancelProjectWait'),
  openRemote: (profileId) => ipcRenderer.invoke('prime:openRemote', profileId),
  remote: (request) => ipcRenderer.invoke('prime:remote', request),
  remoteKey: () => ipcRenderer.invoke('prime:remoteKey'),
  closeProject: (id) => ipcRenderer.invoke('prime:closeProject', id),
  call: (id, operation, args) => ipcRenderer.invoke('prime:call', id, operation, args),
  files: (id, path) => ipcRenderer.invoke('prime:files', id, path),
  saveFile: (id, path, content, revision) =>
    ipcRenderer.invoke('prime:saveFile', id, path, content, revision),
  editorDirty: (dirty) => ipcRenderer.invoke('prime:editorDirty', dirty),
  readFile: (id, path) => ipcRenderer.invoke('prime:readFile', id, path),
  diff: (id, session, path, source) => ipcRenderer.invoke('prime:diff', id, session, path, source),
  attachments: () => ipcRenderer.invoke('prime:attachments'),
  preferences: (patch) => ipcRenderer.invoke('prime:preferences', patch),
  openExternal: (url) => ipcRenderer.invoke('prime:openExternal', url),
  reveal: (id, path) => ipcRenderer.invoke('prime:reveal', id, path),
  createEntry: (id, path, kind) => ipcRenderer.invoke('prime:createEntry', id, path, kind),
  trashEntry: (id, path) => ipcRenderer.invoke('prime:trashEntry', id, path),
  openWith: (id, path, target) => ipcRenderer.invoke('prime:openWith', id, path, target),
  browserShow: (bounds) => ipcRenderer.invoke('prime:browserShow', bounds),
  browserHide: () => ipcRenderer.invoke('prime:browserHide'),
  browserTab: (action, id, url) => ipcRenderer.invoke('prime:browserTab', action, id, url),
  browserState: () => ipcRenderer.invoke('prime:browserState'),
  browserNavigate: (url) => ipcRenderer.invoke('prime:browserNavigate', url),
  browserCommand: (command) => ipcRenderer.invoke('prime:browserCommand', command),
  browserOpenExternal: () => ipcRenderer.invoke('prime:browserOpenExternal'),
  update: (action) => ipcRenderer.invoke('prime:update', action),
  onEvent: (listener) => {
    const handler = (_event: Electron.IpcRendererEvent, value: any) => listener(value)
    ipcRenderer.on('prime:event', handler)
    return () => ipcRenderer.removeListener('prime:event', handler)
  }
}
contextBridge.exposeInMainWorld('prime', api)
