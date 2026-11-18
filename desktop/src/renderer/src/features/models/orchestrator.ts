import type {
  OrchestratorConfig,
  OrchestratorPhase,
  OrchestratorRoleConfig
} from '../../../../shared/contracts'
import { reasoningLabel } from './reasoning'
import { findCapability, type ModelCapability } from './useModelCapabilities'

export type OrchestratorRole = 'planner' | 'executor'

export const PLANNER_REASONING_LEVELS = ['low', 'medium', 'high', 'xhigh', 'max'] as const
export const EXECUTOR_REASONING_LEVELS = ['off', 'low', 'medium', 'high', 'xhigh'] as const

export function roleReasoningOptions(
  role: OrchestratorRole,
  capability?: ModelCapability
): string[] {
  const candidates =
    role === 'planner' ? [...PLANNER_REASONING_LEVELS] : [...EXECUTOR_REASONING_LEVELS]
  if (capability?.reasoning_levels?.length) {
    const allowed = new Set(capability.reasoning_levels)
    const filtered = candidates.filter((level) => allowed.has(level))
    if (filtered.length > 0) return filtered
  }
  if (capability && capability.can_reason === false) {
    return role === 'executor' ? ['off'] : []
  }
  return candidates
}

export function resolveRoleConfig(
  role: OrchestratorRole,
  config: any,
  sessionId?: string
): OrchestratorRoleConfig | undefined {
  const orch: OrchestratorConfig | undefined = config?.orchestrator
  const explicit = role === 'planner' ? orch?.planner : orch?.executor
  if (explicit?.provider && explicit?.model) return explicit
  const sessionModel = sessionId ? config?.session_models?.[sessionId] : undefined
  if (role === 'planner') {
    const fallback = sessionModel ?? config?.models?.large
    return fallback?.provider && fallback?.model
      ? { provider: fallback.provider, model: fallback.model }
      : undefined
  }
  const fallback = config?.models?.small ?? config?.models?.large
  return fallback?.provider && fallback?.model
    ? { provider: fallback.provider, model: fallback.model }
    : undefined
}

export function resolveRoleReasoning(
  role: OrchestratorRole,
  roleConfig: OrchestratorRoleConfig | undefined,
  capability?: ModelCapability
): string {
  const options = roleReasoningOptions(role, capability)
  const explicit = roleConfig?.reasoning_mode
  if (explicit && (!options.length || options.includes(explicit))) return explicit
  const capDefault = capability?.default_reasoning_mode
  if (capDefault && (!options.length || options.includes(capDefault))) return capDefault
  if (role === 'planner') {
    if (options.includes('high')) return 'high'
    if (options.includes('medium')) return 'medium'
    return options[0] || 'medium'
  }
  if (options.includes('medium')) return 'medium'
  if (options.includes('low')) return 'low'
  return options[0] || 'off'
}

export function resolveRoleDisplay(
  role: OrchestratorRole,
  config: any,
  sessionId: string | undefined,
  models: ModelCapability[]
) {
  const ref = resolveRoleConfig(role, config, sessionId)
  const capability = ref ? findCapability(models, ref) : undefined
  const name = capability?.name || ref?.model || ''
  const provider = capability?.provider_name || ref?.provider || ''
  const reasoning = resolveRoleReasoning(role, ref, capability)
  const reasoningOptions = roleReasoningOptions(role, capability)
  return { ref, capability, name, provider, reasoning, reasoningOptions }
}

export function compactModelLabel(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return ''
  return trimmed.replace(/^Claude\s+/i, '').replace(/^Gemini\s+/i, '')
}

export function compactReasoningLabel(mode: string, t: (key: string) => string): string {
  if (mode === 'off') return t('Выкл')
  if (mode === 'xhigh') return t('Оч. выс.')
  if (mode === 'max') return t('Макс.')
  return reasoningLabel(mode, t)
}

export function orchestratorPhaseLabel(
  phase: OrchestratorPhase,
  t: (key: string) => string
): string {
  if (phase === 'plan') return t('Планировщик')
  if (phase === 'execute') return t('Исполнитель')
  return t('Проверка')
}
