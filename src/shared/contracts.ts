export type Json = any
export type Session = {
  id: string
  title: string
  parent_session_id?: string
  is_busy?: boolean
  cost?: number
  prompt_tokens?: number
  completion_tokens?: number
  updated_at?: number
  todos?: Json[]
}
export type Message = {
  id: string
  session_id: string
  role: string
  parts: { type: string; data: Json }[]
  created_at?: number
  updated_at?: number
}
export type Project = {
  kind?: 'chat' | 'project'
  id: string
  path: string
  clientId: string
  connection: string
  currentSessionId?: string
  channels?: string[]
}
export type DesktopEvent = { workspaceId: string; type: string; payload: Json }
export type Preferences = {
  theme: 'dark' | 'light' | 'system'
  language: 'ru' | 'en'
  recent: string[]
  pinned: string[]
  drafts: Record<string, string>
  sidebarWidth: number
  inspectorWidth: number
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
  | 'history'
  | 'agent'
  | 'init'
  | 'send'
  | 'cancel'
  | 'mode'
  | 'queue'
  | 'clearQueue'
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
  | 'voice'
  | 'pending'
  | 'runs'
  | 'commands'
  | 'jobs'
  | 'currentSession'
  | 'importCopilot'
  | 'refreshOAuth'
export interface DesktopApi {
  changes(workspaceId: string): Promise<{ path: string; status: string; originalPath?: string }[]>
  info(): Promise<Json>
  projects(): Promise<Project[]>
  openChat(): Promise<Project>
  openProject(path?: string): Promise<Project | null>
  closeProject(id: string): Promise<void>
  call(workspaceId: string, operation: Operation, args?: Json): Promise<Json>
  files(workspaceId: string, path?: string): Promise<Json[]>
  readFile(workspaceId: string, path: string): Promise<Json>
  diff(
    workspaceId: string,
    sessionId: string,
    path: string,
    source: 'session' | 'git'
  ): Promise<Json>
  attachments(): Promise<Json[]>
  preferences(patch?: Partial<Preferences>): Promise<Preferences>
  openExternal(url: string): Promise<void>
  reveal(workspaceId: string, path?: string): Promise<void>
  update(action: 'check' | 'install'): Promise<Json>
  onEvent(listener: (event: DesktopEvent) => void): () => void
}
declare global {
  interface Window {
    prime: DesktopApi
  }
}
