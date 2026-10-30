export type Json = any
export type BrowserState = {
  url: string
  title: string
  loading: boolean
  canGoBack: boolean
  canGoForward: boolean
  activeTab?: string
  tabs?: { id: string; url: string; title: string; loading: boolean }[]
}
export type BrowserLog = {
  level: 'error' | 'warning' | 'info'
  message: string
  source?: string
  line?: number
  url: string
  time: number
}
export type PickedElementResult = {
  element: {
    selector: string
    tag: string
    text: string
    html: string
    url: string
    rect: { x: number; y: number; width: number; height: number }
    styles: Record<string, string>
  }
  attachmentId?: string
  image?: string
}
export type QueuedPrompt = { id: string; prompt: string; attachments: number; steering?: boolean }
export type Session = {
  id: string
  title: string
  parent_session_id?: string
  is_busy?: boolean
  cost?: number
  prompt_tokens?: number
  completion_tokens?: number
  created_at?: number
  updated_at?: number
  todos?: Json[]
}
export type Message = {
  id: string
  session_id: string
  role: string
  parts: { type: string; data: Json }[]
  is_summary_message?: boolean
  created_at?: number
  updated_at?: number
}
export type MessagePage = {
  messages: Message[]
  before?: string
  after?: string
  has_more: boolean
  has_before: boolean
  has_after: boolean
  timing: { started_at?: number; completed?: Record<string, number> }
}
export type Project = {
  remote?: {
    profileId: string
    name: string
    host: string
    port: number
    user: string
    root: string
  }
  kind?: 'chat' | 'project'
  id: string
  path: string
  clientId: string
  connection: string
  currentSessionId?: string
  channels?: string[]
}
export type DesktopEvent = { workspaceId: string; type: string; payload: Json }
export type ChatReasoningMode = 'off' | 'low' | 'medium' | 'high' | 'xhigh' | 'max' | 'default'
export type Preferences = {
  theme: 'dark' | 'light' | 'system'
  language: 'ru' | 'en'
  recent: string[]
  projectOrder?: string[]
  sessionOrder?: Record<string, string[]>
  pinned: string[]
  favoriteModels?: { provider: string; model: string }[]
  recentModels?: { provider: string; model: string }[]
  reasoningBySession?: Record<string, ChatReasoningMode>
  lastReasoningMode?: ChatReasoningMode
  agentMode?: 'general' | 'plan'
  drafts: Record<string, string>
  sidebarWidth: number
  inspectorWidth: number
  density?: 'comfortable' | 'compact'
  reduceMotion?: boolean
  sidebarVisible?: boolean
  speechVoice?: string
  speechSpeed?: number
  speechAutoSend?: boolean
  micDeviceId?: string
  speakerDeviceId?: string
  inspectorVisible?: boolean
  keybindings?: Record<string, string>
  agentContexts?: Record<string, import('./agentContext').AgentContext>
  skipBySession?: Record<string, boolean>
  defaultSkip?: boolean
  activeProject?: string
  activeSessions?: Record<string, string>
  projectChannels?: Record<string, string[]>
}
export type Operation =
  | 'channelMessages'
  | 'sessions'
  | 'createSession'
  | 'renameSession'
  | 'deleteSession'
  | 'messages'
  | 'messagePage'
  | 'messageSearch'
  | 'history'
  | 'context'
  | 'agent'
  | 'agentWorkers'
  | 'agentWorkerMessages'
  | 'agentWorkerSend'
  | 'agentWorkerStop'
  | 'init'
  | 'send'
  | 'cancel'
  | 'mode'
  | 'queue'
  | 'clearQueue'
  | 'runQueuedNow'
  | 'summarize'
  | 'shell'
  | 'permission'
  | 'skip'
  | 'getSkip'
  | 'answer'
  | 'cancelQuestion'
  | 'config'
  | 'providers'
  | 'configSet'
  | 'configRemove'
  | 'model'
  | 'providerKey'
  | 'providerSave'
  | 'reasoning'
  | 'models'
  | 'mcp'
  | 'lsp'
  | 'skills'
  | 'readSkill'
  | 'restart'
  | 'mcpPrompts'
  | 'mcpPrompt'
  | 'mcpResource'
  | 'mcpRefresh'
  | 'mcpAuth'
  | 'mcpAuthUrl'
  | 'lspStart'
  | 'lspStop'
  | 'diagnostics'
  | 'dockerEnable'
  | 'dockerDisable'
  | 'plugins'
  | 'pluginEnable'
  | 'pluginDisable'
  | 'pluginTasks'
  | 'stt'
  | 'sttInstall'
  | 'sttRemove'
  | 'sttSelect'
  | 'sttTasks'
  | 'voice'
  | 'pending'
  | 'runs'
  | 'commands'
  | 'jobs'
  | 'currentSession'
  | 'importCopilot'
  | 'refreshOAuth'
export interface DesktopApi {
  automation(
    workspaceId: string,
    request: import('./automations').AutomationRequest
  ): Promise<import('./automations').AutomationResponse>
  launch(
    workspaceId: string,
    request: import('./launch').LaunchRequest
  ): Promise<import('./launch').LaunchResponse>
  browserLogs(clear?: boolean): Promise<BrowserLog[]>
  speech(request: import('./speech').SpeechRequest): Promise<any>
  github(workspaceId: string, request: import('./github').GithubRequest): Promise<any>
  windows(workspaceId: string, request: import('./computer').WindowRequest): Promise<any>
  jobs(request: import('./jobs').JobRequest): Promise<import('./jobs').Job[]>
  browserPick(): Promise<PickedElementResult[] | null>
  browserPickCancel(): Promise<void>
  team(
    workspaceId: string,
    request: import('./teams').TeamRequest
  ): Promise<import('./teams').TeamResponse>
  terminal(
    workspaceId: string,
    request: import('./terminal').TerminalRequest
  ): Promise<import('./terminal').TerminalResponse>
  changes(workspaceId: string): Promise<{ path: string; status: string; originalPath?: string }[]>
  info(): Promise<Json>
  projects(): Promise<Project[]>
  projectSessions(paths: string[]): Promise<Record<string, Session[]>>
  openChat(): Promise<Project>
  openProject(path?: string): Promise<Project | null>
  cancelProjectWait(): Promise<void>
  openRemote(profileId: string): Promise<Project>
  remote(request: import('./remote').RemoteRequest): Promise<import('./remote').RemoteSnapshot>
  remoteKey(): Promise<string | undefined>
  sudoPassword(
    request: { action: 'status' } | { action: 'set'; password: string } | { action: 'remove' }
  ): Promise<{ hasPassword: boolean; storageAvailable: boolean }>
  deleteProject(path: string): Promise<void>
  call(workspaceId: string, operation: Operation, args?: Json): Promise<Json>
  files(workspaceId: string, path?: string): Promise<Json[]>
  saveFile(
    workspaceId: string,
    path: string,
    content: string,
    revision: string
  ): Promise<{ revision: string }>
  editorDirty(dirty: boolean): Promise<void>
  readFile(workspaceId: string, path: string): Promise<Json>
  diff(
    workspaceId: string,
    sessionId: string,
    path: string,
    source: 'session' | 'git'
  ): Promise<Json>
  attachments(): Promise<Json[]>
  importImages(
    images: import('./attachments').ImageUpload[]
  ): Promise<import('./attachments').Attachment[]>
  releaseAttachments(ids: string[]): Promise<void>
  preferences(patch?: Partial<Preferences>): Promise<Preferences>
  openExternal(url: string): Promise<void>
  windowDimmed(dimmed: boolean): Promise<void>
  reveal(workspaceId: string, path?: string): Promise<void>
  createEntry(workspaceId: string, path: string, kind: 'file' | 'folder'): Promise<{ path: string }>
  trashEntry(workspaceId: string, path: string): Promise<void>
  openWith(workspaceId: string, path: string, target: 'default' | 'vscode'): Promise<void>
  browserShow(bounds: { x: number; y: number; width: number; height: number }): Promise<void>
  browserHide(): Promise<void>
  browserTab(action: 'new' | 'select' | 'close', id?: string, url?: string): Promise<BrowserState>
  browserState(): Promise<BrowserState>
  browserNavigate(url: string): Promise<BrowserState>
  browserCommand(command: 'back' | 'forward' | 'reload'): Promise<BrowserState>
  browserOpenExternal(): Promise<void>
  update(action: 'check' | 'install'): Promise<Json>
  onEvent(listener: (event: DesktopEvent) => void): () => void
}
declare global {
  interface Window {
    prime: DesktopApi
  }
}
