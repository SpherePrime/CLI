export const openAgentEvent = 'prime:open-agent'

export type AgentRequest = { workerId: string; sequence: number }

export function agentWorkerId(messageId: string, toolCallId: string) {
  return `${messageId}$$${toolCallId}`
}

export function openAgent(workerId: string) {
  window.dispatchEvent(new CustomEvent(openAgentEvent, { detail: { workerId } }))
}
