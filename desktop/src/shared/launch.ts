export type LaunchConfig = { id: string; name: string; command: string; url?: string }
export type LaunchConfigInput = Omit<LaunchConfig, 'id'> & { id?: string }

export type LaunchStatus = 'stopped' | 'running' | 'exited'

export type LaunchProcess = {
  configId: string
  terminalId: string
  status: LaunchStatus
  startedAt: number
  url?: string
  exitCode?: number
}

export type LaunchResponse = { root: string; configs: LaunchConfig[]; processes: LaunchProcess[] }

export type LaunchRequest =
  | { action: 'list' }
  | { action: 'save'; input: LaunchConfigInput }
  | { action: 'remove' | 'start' | 'stop' | 'restart'; id: string }

export type LaunchApi = (request: LaunchRequest) => Promise<LaunchResponse>
