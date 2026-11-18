export type CustomAgentRole = 'orchestrator' | 'worker' | 'verifier'

export type CustomAgentTriggerType = 'manual' | 'interval' | 'cron' | 'event' | 'monitor'

export type CustomAgentMonitorKind = '' | 'command' | 'http' | 'file'

export type CustomAgentStatus =
  | 'idle'
  | 'monitoring'
  | 'running'
  | 'verifying'
  | 'rework'
  | 'paused'
  | 'error'

export type CustomAgentModelMode = 'auto' | 'manual'

export type CustomAgentReasoningMode =
  | ''
  | 'off'
  | 'low'
  | 'medium'
  | 'high'
  | 'xhigh'
  | 'max'
  | 'default'

export type CustomAgentRunStatus =
  | 'running'
  | 'verifying'
  | 'rework'
  | 'passed'
  | 'failed'
  | 'skipped_no_change'
  | 'cancelled'

export type DeterministicCheckKind = 'command' | 'regex' | 'json_fields' | 'file_exists'

export interface DeterministicCheck {
  id: string
  kind: DeterministicCheckKind
  target: string
  pattern: string
  expected: string
  required: boolean
}

export interface CustomAgent {
  id: string
  slug: string
  name: string
  description: string
  role: CustomAgentRole
  system_prompt: string
  memory: string
  model_mode: CustomAgentModelMode
  provider_id: string
  model_id: string
  reasoning_mode: string
  auto_model_hint: string
  allowed_tools: string[]
  workspace_path: string
  workflow_id?: string
  enabled: boolean
  created_by: string
  trigger_type: CustomAgentTriggerType
  interval_minutes: number
  cron_expr: string
  monitor_kind: CustomAgentMonitorKind
  monitor_target: string
  monitor_last_hash: string
  monitor_last_snapshot: string
  acceptance_criteria: string
  deterministic_checks: DeterministicCheck[]
  verifier_agent_id: string
  next_agent_id: string
  max_rework_attempts: number
  strict_verify: boolean
  status: CustomAgentStatus
  last_error: string
  last_run_at: number
  next_run_at: number
  created_at: number
  updated_at: number
}

export interface RunStep {
  kind: string
  title: string
  detail: string
  status: string
  timestamp: number
}

export interface CustomAgentRun {
  id: string
  agent_id: string
  agent_name: string
  agent_role: CustomAgentRole | string
  chain_id: string
  parent_run_id: string
  trigger_reason: string
  input_prompt: string
  status: CustomAgentRunStatus
  attempt: number
  resolved_provider: string
  resolved_model: string
  resolved_reasoning: string
  auto_model_reason: string
  output_text: string
  verifier_verdict: string
  verifier_feedback: string
  deterministic_report: string
  monitor_diff: string
  steps: RunStep[]
  error: string
  started_at: number
  finished_at: number
}

export interface BlackboardEntry {
  id: string
  chain_id: string
  run_id: string
  from_agent_id: string
  from_agent_name: string
  to_agent_id: string
  topic: string
  payload: string
  status: string
  created_at: number
}

export interface ModelOption {
  provider_id: string
  provider_name: string
  model_id: string
  name: string
  tier: string
  can_reason: boolean
  reasoning_modes: string[]
  reasoning_levels: string[]
  default_reasoning_mode: string
  context_window: number
  cost_per_1m_in: number
  cost_per_1m_out: number
  locked: boolean
  plan_label: string
}

export interface ProbeResult {
  changed: boolean
  hash: string
  snapshot: string
  diff: string
  error: string
}

export interface CustomWorkflow {
  id: string
  slug: string
  name: string
  description: string
  enabled: boolean
  created_by: string
  entry_agent_id: string
  layout_json: string
  created_at: number
  updated_at: number
}

export interface CreateWorkflowInput {
  slug?: string
  name: string
  description?: string
  enabled?: boolean
  created_by?: string
  entry_agent_id?: string
  layout_json?: string
}

export interface UpdateWorkflowPatch {
  name?: string
  description?: string
  enabled?: boolean
  entry_agent_id?: string
  layout_json?: string
}

export interface CustomAgentEvent {
  type: string
  agent_id: string
  run_id: string
  chain_id: string
  workflow_id?: string
  workflow?: CustomWorkflow
  payload: any
  timestamp: number
}

export interface GlobalMemoryDoc {
  memory: string
  user_profile: string
  soul: string
  memory_limit: number
  user_limit: number
  memory_chars: number
  user_chars: number
}

export interface SwarmWorkerSpec {
  agent_id?: string
  slug?: string
  name?: string
  role?: CustomAgentRole | string
  task?: string
  prompt?: string
  system_prompt?: string
  model_mode?: CustomAgentModelMode | string
  provider_id?: string
  model_id?: string
  reasoning_mode?: string
  auto_model_hint?: string
  allowed_tools?: string[]
  workspace_path?: string
  acceptance_criteria?: string
  deterministic_checks?: DeterministicCheck[]
  verifier_agent_id?: string
  max_rework_attempts?: number
  strict_verify?: boolean
}

export interface SwarmRequest {
  goal: string
  chain_id?: string
  workers: SwarmWorkerSpec[]
  acceptance_criteria?: string
  deterministic_checks?: DeterministicCheck[]
  verifier_agent_id?: string
  synthesizer_agent_id?: string
  max_rework_attempts?: number
  persist_workers?: boolean
  skip_verify?: boolean
  skip_synthesize?: boolean
}

export interface SwarmWorkerResult {
  agent_id: string
  slug: string
  name: string
  task: string
  run: CustomAgentRun
  output: string
  status: CustomAgentRunStatus | string
  error?: string
}

export interface SwarmResult {
  chain_id: string
  goal: string
  status: CustomAgentRunStatus | string
  attempts: number
  workers: SwarmWorkerResult[]
  verifier_verdict: string
  verifier_feedback: string
  deterministic_report: string
  final_output: string
  synthesizer_run?: CustomAgentRun | null
  blackboard?: BlackboardEntry[]
  error?: string
}

export type CreateCustomAgentInput = Partial<CustomAgent> & {
  strict_verify?: boolean
  workflow_id?: string
}

export type UpdateCustomAgentPatch = Partial<CustomAgent> & {
  strict_verify?: boolean
  workflow_id?: string
}

export type UpdateCustomAgentInput = UpdateCustomAgentPatch

export type CustomAgentsRequest =
  | { action: 'list' }
  | { action: 'get'; agentId: string }
  | { action: 'create'; input: CreateCustomAgentInput }
  | { action: 'update'; agentId: string; patch: UpdateCustomAgentPatch }
  | { action: 'delete'; agentId: string }
  | {
      action: 'run'
      agentId: string
      prompt: string
      triggerReason?: string
      chainId?: string
      force?: boolean
      sync?: boolean
    }
  | { action: 'cancelRun'; agentId: string; runId: string }
  | { action: 'probe'; agentId: string }
  | { action: 'runs'; agentId?: string; limit?: number }
  | { action: 'blackboard'; chainId?: string; limit?: number }
  | {
      action: 'postBlackboard'
      input: {
        chain_id: string
        from_agent_id?: string
        from_agent_name?: string
        to_agent_id?: string
        topic: string
        payload: string
      }
    }
  | { action: 'models' }
  | { action: 'getGlobalMemory' }
  | { action: 'saveGlobalMemory'; input: Partial<GlobalMemoryDoc> }
  | { action: 'dispatchSwarm'; input: SwarmRequest }
  | { action: 'listWorkflows' }
  | { action: 'createWorkflow'; input: CreateWorkflowInput }
  | { action: 'updateWorkflow'; workflowId: string; patch: UpdateWorkflowPatch }
  | { action: 'deleteWorkflow'; workflowId: string; deleteAgents?: boolean }
  | { action: 'runWorkflow'; workflowId: string; prompt?: string }
