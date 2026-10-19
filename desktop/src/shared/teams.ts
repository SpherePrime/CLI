export type TeamTaskStatus =
  | 'queued'
  | 'starting'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'interrupted'

export type TeamTaskInput = { title: string; instruction: string }

export type TeamTask = TeamTaskInput & {
  id: string
  status: TeamTaskStatus
  runId?: string
  workspaceId?: string
  sessionId?: string
  error?: string
  startedAt?: number
  finishedAt?: number
}

export type Team = {
  id: string
  root: string
  title: string
  goal: string
  concurrency: number
  createdAt: number
  tasks: TeamTask[]
}

export type TeamInput = {
  title: string
  goal: string
  concurrency: number
  tasks: TeamTaskInput[]
}

export type TeamResponse = { root: string; teams: Team[]; error?: string }

export type TeamRequest =
  | { action: 'list' }
  | { action: 'create'; input: TeamInput }
  | { action: 'remove'; teamId: string }
  | { action: 'stopTeam'; teamId: string }
  | { action: 'stopTask' | 'retryTask'; teamId: string; taskId: string }

export type TeamApi = (request: TeamRequest) => Promise<TeamResponse>

export const finishedTeamStatuses: TeamTaskStatus[] = ['completed', 'failed', 'cancelled', 'interrupted']
