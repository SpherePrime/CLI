export type AutomationTrigger =
  | { type: 'interval'; minutes: number }
  | { type: 'daily'; time: string }
  | { type: 'weekly'; time: string; days: number[] }
  | { type: 'filesChanged' | 'runFailed'; cooldownMinutes: number }

export type AutomationInput = {
  id?: string
  title: string
  prompt: string
  enabled: boolean
  trigger: AutomationTrigger
}
export type AutomationRule = AutomationInput & {
  id: string
  root: string
  createdAt: number
  updatedAt: number
  nextAt?: number
  lastTriggeredAt?: number
  lastFingerprint?: string
}
export type AutomationStatus =
  'claimed' | 'running' | 'completed' | 'failed' | 'cancelled' | 'interrupted'
export type AutomationRun = {
  id: string
  ruleId: string
  root: string
  title: string
  trigger: 'manual' | AutomationTrigger['type']
  status: AutomationStatus
  createdAt: number
  finishedAt?: number
  workspaceId?: string
  sessionId?: string
  error?: string
}
export type AutomationResponse = {
  root: string
  rules: AutomationRule[]
  runs: AutomationRun[]
  error?: string
}
export type AutomationRequest =
  | { action: 'list'; root: string }
  | { action: 'save'; root: string; input: AutomationInput }
  | { action: 'remove' | 'run'; root: string; id: string }
  | { action: 'cancel'; root: string; runId: string }
export type AutomationApi = (request: AutomationRequest) => Promise<AutomationResponse>
export type AutomationEvent = {
  root: string
  type: 'filesChanged' | 'runFailed'
  sourceRunId?: string
  fingerprint?: string
}
