import type {
  CustomAgent,
  CustomAgentModelMode,
  CustomAgentRole,
  CustomAgentRun,
  CustomAgentRunStatus,
  CustomAgentStatus,
  CustomAgentTriggerType,
  DeterministicCheck,
  ModelOption
} from '../../../../shared/customAgents'

export type RoleFilter = 'all' | CustomAgentRole

export type StatusTone = 'neutral' | 'accent' | 'green' | 'amber' | 'red'

export interface AgentPreset {
  id: string
  label: string
  role: CustomAgentRole
  name: string
  description: string
  trigger_type: CustomAgentTriggerType
  interval_minutes: number
  monitor_kind: '' | 'command' | 'http' | 'file'
  monitor_target: string
  auto_model_hint: string
  system_prompt: string
  acceptance_criteria: string
  deterministic_checks: DeterministicCheck[]
}

export const AGENT_PRESETS: AgentPreset[] = [
  {
    id: 'monitor',
    label: 'Монитор изменений',
    role: 'worker',
    name: 'Монитор изменений',
    description: 'Отслеживает изменения источника по расписанию и передаёт дифф на анализ.',
    trigger_type: 'monitor',
    interval_minutes: 15,
    monitor_kind: 'http',
    monitor_target: 'https://example.com/status.json',
    auto_model_hint: 'fast',
    system_prompt:
      'Сравни новый снимок с предыдущим, выдели ключевые изменения и подготовь краткую сводку.',
    acceptance_criteria: 'Сводка должна отражать суть изменений без вымышленных деталей.',
    deterministic_checks: []
  },
  {
    id: 'researcher',
    label: 'Исследователь',
    role: 'worker',
    name: 'Исследователь',
    description: 'Анализирует материалы, собирает факты и формирует структурированный отчёт.',
    trigger_type: 'manual',
    interval_minutes: 0,
    monitor_kind: '',
    monitor_target: '',
    auto_model_hint: 'reasoning',
    system_prompt:
      'Проведи глубокий разбор задачи, отдели подтверждённые факты от гипотез и оформи выводы.',
    acceptance_criteria: 'Чёткие выводы со ссылками на проверенные источники или файлы.',
    deterministic_checks: []
  },
  {
    id: 'verifier',
    label: 'Строгий верификатор',
    role: 'verifier',
    name: 'Строгий верификатор',
    description: 'Проверяет результаты других агентов по критериям приёмки и детерминированным правилам.',
    trigger_type: 'manual',
    interval_minutes: 0,
    monitor_kind: '',
    monitor_target: '',
    auto_model_hint: 'reasoning',
    system_prompt:
      'Проверяй результат строго по критериям приёмки. Начинай ответ с VERDICT: PASS или VERDICT: FAIL.',
    acceptance_criteria: '',
    deterministic_checks: []
  },
  {
    id: 'orchestrator',
    label: 'Координатор',
    role: 'orchestrator',
    name: 'Координатор цепочки',
    description: 'Декомпозирует задачи, управляет цепочкой исполнителей и координирует общую доску.',
    trigger_type: 'manual',
    interval_minutes: 0,
    monitor_kind: '',
    monitor_target: '',
    auto_model_hint: 'flagship',
    system_prompt:
      'Разбивай сложную задачу на шаги, назначай исполнителей и фиксируй решения на доске.',
    acceptance_criteria: 'Все подзадачи распределены, зависимости зафиксированы.',
    deterministic_checks: []
  }
]

export const ORCHESTRATOR_QUICK_PROMPTS = [
  'Создай агента мониторинга релизов и свяжи его со строгим верификатором',
  'Проанализируй текущую цепочку и распредели подзадачи на доске',
  'Собери сводку по последним запускам и предложи улучшения проверок'
]

export function agentRoleLabel(role: string): string {
  switch (role) {
    case 'orchestrator':
      return 'Координатор'
    case 'verifier':
      return 'Верификатор'
    default:
      return 'Воркер'
  }
}

export function agentStatusMeta(status: CustomAgentStatus | string): {
  label: string
  tone: StatusTone
} {
  switch (status) {
    case 'monitoring':
      return { label: 'Мониторит', tone: 'accent' }
    case 'running':
      return { label: 'Работает', tone: 'accent' }
    case 'verifying':
      return { label: 'Проверка', tone: 'amber' }
    case 'rework':
      return { label: 'Доработка', tone: 'amber' }
    case 'paused':
      return { label: 'Пауза', tone: 'neutral' }
    case 'error':
      return { label: 'Ошибка', tone: 'red' }
    default:
      return { label: 'Свободен', tone: 'green' }
  }
}

export function runStatusMeta(status: CustomAgentRunStatus | string): {
  label: string
  tone: StatusTone
} {
  switch (status) {
    case 'running':
      return { label: 'Выполняется', tone: 'accent' }
    case 'verifying':
      return { label: 'Проверка', tone: 'amber' }
    case 'rework':
      return { label: 'Доработка', tone: 'amber' }
    case 'passed':
      return { label: 'Пройден', tone: 'green' }
    case 'failed':
      return { label: 'Ошибка', tone: 'red' }
    case 'skipped_no_change':
      return { label: 'Без изменений', tone: 'neutral' }
    case 'cancelled':
      return { label: 'Отменён', tone: 'neutral' }
    default:
      return { label: status || 'Неизвестно', tone: 'neutral' }
  }
}

export function isActiveRunStatus(status: string): boolean {
  return status === 'running' || status === 'verifying' || status === 'rework'
}

export function triggerLabel(agent: Pick<CustomAgent, 'trigger_type' | 'interval_minutes' | 'cron_expr' | 'monitor_kind'>): string {
  switch (agent.trigger_type) {
    case 'interval':
      return agent.interval_minutes > 0 ? `Каждые ${agent.interval_minutes} мин` : 'По интервалу'
    case 'cron':
      return agent.cron_expr ? `Cron: ${agent.cron_expr}` : 'По расписанию Cron'
    case 'event':
      return 'По событию'
    case 'monitor': {
      const kind =
        agent.monitor_kind === 'http'
          ? 'HTTP'
          : agent.monitor_kind === 'file'
            ? 'Файл'
            : agent.monitor_kind === 'command'
              ? 'Команда'
              : 'Зонд'
      return agent.interval_minutes > 0
        ? `Монитор (${kind}) · ${agent.interval_minutes} мин`
        : `Монитор (${kind})`
    }
    default:
      return 'Вручную'
  }
}

export function modelSummary(
  agent: Pick<CustomAgent, 'model_mode' | 'provider_id' | 'model_id'>,
  models: ModelOption[]
): string {
  if (agent.model_mode !== 'manual' || !agent.model_id) {
    return 'Авто'
  }
  const match = models.find(
    (m) => m.model_id === agent.model_id && (!agent.provider_id || m.provider_id === agent.provider_id)
  )
  return match?.name || agent.model_id
}

export function filterCustomAgents(
  agents: CustomAgent[],
  query: string,
  role: RoleFilter
): CustomAgent[] {
  const needle = query.trim().toLowerCase()
  return agents.filter((agent) => {
    if (role !== 'all' && agent.role !== role) return false
    if (!needle) return true
    return (
      agent.name.toLowerCase().includes(needle) ||
      agent.slug.toLowerCase().includes(needle) ||
      agent.description.toLowerCase().includes(needle)
    )
  })
}

export function formatTimestamp(unixSeconds: number, locale = 'ru-RU'): string {
  if (!unixSeconds) return '—'
  const date = new Date(unixSeconds * 1000)
  return date.toLocaleString(locale, {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

export function formatDuration(startedAt: number, finishedAt: number): string {
  if (!startedAt) return ''
  const end = finishedAt > 0 ? finishedAt : Math.floor(Date.now() / 1000)
  const seconds = Math.max(0, end - startedAt)
  if (seconds < 60) return `${seconds} с`
  const minutes = Math.floor(seconds / 60)
  const rem = seconds % 60
  return rem > 0 ? `${minutes} м ${rem} с` : `${minutes} м`
}

export interface AgentDraft {
  name: string
  slug: string
  role: CustomAgentRole
  description: string
  workspace_path: string
  workflow_id: string
  enabled: boolean
  model_mode: CustomAgentModelMode
  provider_id: string
  model_id: string
  reasoning_mode: string
  auto_model_hint: string
  system_prompt: string
  memory: string
  trigger_type: CustomAgentTriggerType
  interval_minutes: number
  cron_expr: string
  monitor_kind: '' | 'command' | 'http' | 'file'
  monitor_target: string
  acceptance_criteria: string
  deterministic_checks: DeterministicCheck[]
  verifier_agent_id: string
  next_agent_id: string
  max_rework_attempts: number
  strict_verify: boolean
}

export function buildAgentDraft(agent: CustomAgent): AgentDraft {
  return {
    name: agent.name,
    slug: agent.slug,
    role: agent.role,
    description: agent.description,
    workspace_path: agent.workspace_path,
    workflow_id: agent.workflow_id || 'default',
    enabled: Boolean(agent.enabled),
    model_mode: agent.model_mode || 'auto',
    provider_id: agent.provider_id,
    model_id: agent.model_id,
    reasoning_mode: agent.reasoning_mode,
    auto_model_hint: agent.auto_model_hint,
    system_prompt: agent.system_prompt,
    memory: agent.memory,
    trigger_type: agent.trigger_type || 'manual',
    interval_minutes: agent.interval_minutes || 0,
    cron_expr: agent.cron_expr,
    monitor_kind: agent.monitor_kind || '',
    monitor_target: agent.monitor_target,
    acceptance_criteria: agent.acceptance_criteria,
    deterministic_checks: structuredClone(agent.deterministic_checks || []),
    verifier_agent_id: agent.verifier_agent_id,
    next_agent_id: agent.next_agent_id,
    max_rework_attempts: agent.max_rework_attempts || 3,
    strict_verify: Boolean(agent.strict_verify)
  }
}

export function isAgentDraftDirty(agent: CustomAgent, draft: AgentDraft): boolean {
  return JSON.stringify(buildAgentDraft(agent)) !== JSON.stringify(draft)
}
