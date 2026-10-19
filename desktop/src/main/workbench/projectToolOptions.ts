import type { AutomationManager } from '../automations/manager'
import type { LaunchManager } from '../launch/manager'
import type { TeamManager } from '../teams/manager'
import type { TerminalManager } from '../terminal/manager'
import type { SettingsStore } from '../settings/store'
import type { RemoteConnection } from '../remote/connection'
import type { WorkbenchTool } from './tools'
import type { AgentContext } from '../../shared/agentContext'

export type ProjectToolTarget = {
  workspaceId: string
  root: string
  key: string
  remote?: RemoteConnection
}
export type ProjectToolView = 'automations' | 'launch' | 'teams' | 'terminal'
export type ProjectToolOptions = {
  target(workspaceId: string): ProjectToolTarget
  automations(): AutomationManager | undefined
  launches(): LaunchManager | undefined
  teams(): TeamManager | undefined
  terminals(): TerminalManager | undefined
  settings(): Pick<SettingsStore, 'get' | 'update'>
  files(workspaceId: string): { read(path: string): Promise<{ content?: string; image?: string }> }
  reveal?(workspaceId: string, view: ProjectToolView): void
  openSession?(workspaceId: string, sessionId: string): void | Promise<void>
  onContextChanged?(workspaceId: string, context: AgentContext): void
  extensions?: WorkbenchTool[]
}
