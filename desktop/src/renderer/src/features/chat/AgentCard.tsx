import { Bot, Check, ChevronRight, LoaderCircle, Users, X } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { agentWorkerId, openAgent } from '../agents/openAgent'
import './agentCard.css'

function parameters(value: unknown) {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return {}
  }
}

function backgroundLaunch(metadata: unknown, content: unknown) {
  if (typeof content === 'string' && content.startsWith('Background agent job')) return true
  return typeof metadata === 'string' && metadata.includes('"background":true')
}

export function AgentCard({
  part,
  messageId
}: {
  part: Message['parts'][number]
  messageId: string
}) {
  const { t } = useI18n()
  const data = part.data
  const input = (parameters(data.input) || {}) as Record<string, unknown>
  const result = data.result
  const name = String(input.name || '').trim()
  const role = String(input.agent || 'code')
  const task = String(input.prompt || '')
    .replace(/\s+/g, ' ')
    .trim()
  const failed = Boolean(result?.is_error)
  const launched = result !== undefined && backgroundLaunch(result?.metadata, result?.content)
  const finished = result !== undefined && !launched
  const workerId = data.id ? agentWorkerId(messageId, String(data.id)) : ''
  const status = failed
    ? t('Ошибка')
    : finished
      ? t('Готово')
      : launched
        ? t('Работает в команде')
        : t('Выполняется')
  return (
    <button
      type="button"
      className={`tool-card agent-card ${failed ? 'tool-error' : ''}`}
      disabled={!workerId}
      title={t('Открыть чат агента')}
      onClick={() => openAgent(workerId)}
    >
      <span className="tool-icon">
        <Bot size={15} />
      </span>
      <span className="agent-card-text">
        <span className="agent-card-title">
          <strong>{name || t('Агент')}</strong>
          <span className="agent-role">{role}</span>
        </span>
        {task && <span className="agent-card-task">{task}</span>}
      </span>
      <span
        className={`tool-status ${failed ? 'failed' : finished ? 'done' : launched ? 'background' : 'running'}`}
        title={status}
      >
        {failed ? (
          <X size={13} strokeWidth={2.5} />
        ) : finished ? (
          <Check size={13} strokeWidth={2.5} />
        ) : launched ? (
          <Users size={12} strokeWidth={2.5} />
        ) : (
          <LoaderCircle size={13} className="spin" />
        )}
      </span>
      <ChevronRight size={14} className="agent-card-open" />
    </button>
  )
}
