import type { AgentWorker } from '../../../../shared/agents'

export function agentApprovalSessions(rootSessionId: string, workers: AgentWorker[]) {
  const sessions = new Map([[rootSessionId, '']])
  for (const worker of workers) {
    if (worker.root_session_id === rootSessionId && worker.session_id)
      sessions.set(worker.session_id, worker.name || worker.agent)
  }
  return sessions
}
