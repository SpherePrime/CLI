export type JobKind = 'agent' | 'automation' | 'team' | 'terminal' | 'app'

export type JobStatus = 'queued' | 'running' | 'waiting' | 'completed' | 'failed' | 'cancelled' | 'interrupted'

export type Job = {
  id: string
  kind: JobKind
  title: string
  projectKey: string
  projectName: string
  workspaceId?: string
  sessionId?: string
  status: JobStatus
  startedAt?: number
  finishedAt?: number
  error?: string
  detail?: string
  attention?: 'permission' | 'question'
  stoppable: boolean
}

export type JobRequest = { action: 'list' } | { action: 'stop'; id: string }

export const activeJobStatuses: JobStatus[] = ['queued', 'running', 'waiting']
