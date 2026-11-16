import { workerActive, type AgentWorker } from '../../../../shared/agents'

export const openAgentEvent = 'prime:open-agent'

export type AgentRequest = { workerId: string; sequence: number; sessionId?: string }

export function agentWorkerId(messageId: string, toolCallId: string) {
  return `${messageId}$$${toolCallId}`
}

export function openAgent(workerId: string, sessionId?: string) {
  window.dispatchEvent(new CustomEvent(openAgentEvent, { detail: { workerId, sessionId } }))
}

export function scopeAgentRequest(request: AgentRequest | undefined, sessionId: string) {
  return request && (!request.sessionId || request.sessionId === sessionId) ? request : undefined
}

export function consumeAgentRequestState(current: AgentRequest | undefined, sequence: number) {
  return current?.sequence === sequence ? undefined : current
}

export function agentRosterPollInterval(workers: AgentWorker[], selectedWorkerId: string) {
  if (selectedWorkerId && !workers.some((item) => item.id === selectedWorkerId)) return 400
  return workers.some(workerActive) ? 1000 : 3000
}

