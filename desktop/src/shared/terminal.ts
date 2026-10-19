export type TerminalOwner = 'user' | 'agent'
export type TerminalStatus = 'running' | 'exited' | 'closed'

export type TerminalOptions = {
  cols?: number
  rows?: number
  shell?: string
  args?: string[]
  title?: string
}

export type TerminalSnapshot = {
  id: string
  workspaceId: string
  title: string
  shell: string
  cwd: string
  pid: number
  cols: number
  rows: number
  owner: TerminalOwner
  status: TerminalStatus
  createdAt: number
  sequence: number
  exitCode?: number
  signal?: number
}

export type TerminalRead = {
  terminal: TerminalSnapshot
  output: string
  sequence: number
  truncated: boolean
}

export type TerminalDataEvent = { action: 'data'; id: string; data: string; sequence: number }
export type TerminalEvent =
  | TerminalDataEvent
  | { action: 'state'; terminal: TerminalSnapshot }
  | { action: 'exit'; id: string; exitCode: number; signal?: number; sequence: number }

export type TerminalRequest =
  | { action: 'list' }
  | { action: 'create'; options?: TerminalOptions }
  | { action: 'read'; id: string }
  | { action: 'write'; id: string; data: string }
  | { action: 'report'; id: string; data: string }
  | { action: 'resize'; id: string; cols: number; rows: number }
  | { action: 'close'; id: string }
  | { action: 'owner'; id: string; owner: TerminalOwner }

export type TerminalResponse = TerminalSnapshot[] | TerminalSnapshot | TerminalRead | void
export type TerminalApi = (request: TerminalRequest) => Promise<TerminalResponse>
