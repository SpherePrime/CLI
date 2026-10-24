import { useCallback, useState } from 'react'
import { ChevronRight, RefreshCw, Users } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { AgentWorker } from '../../../../shared/agents'
import { workerActive, workerStatusLabels } from '../../../../shared/agents'
import { useI18n } from '../../i18n'
import { useAgentPolling } from './useAgentPolling'
import { WorkerConversation } from './WorkerConversation'
import './agents.css'

export function AgentsPanel({ workspace }: { workspace: Workspace }) {
  const { t } = useI18n()
  if (!workspace.project || !workspace.session)
    return (
      <div className="panel-empty">
        <Users size={28} />
        <p>{t('Откройте чат, чтобы увидеть его агентов')}</p>
      </div>
    )
  return (
    <AgentTeam
      key={`${workspace.project.id}:${workspace.session.id}`}
      projectId={workspace.project.id}
      sessionId={workspace.session.id}
      disabled={!!workspace.config.options?.disable_multi_agent}
    />
  )
}

function AgentTeam({
  projectId,
  sessionId,
  disabled
}: {
  projectId: string
  sessionId: string
  disabled: boolean
}) {
  const { t } = useI18n()
  const [selected, setSelected] = useState('')
  const load = useCallback(async () => {
    const result = await window.prime.call(projectId, 'agentWorkers', { sessionId })
    const workers = (result.workers || []) as AgentWorker[]
    return workers
  }, [projectId, sessionId])
  const roster = useAgentPolling(
    load,
    (workers) => (workers.some(workerActive) ? 1000 : 3000),
    [] as AgentWorker[]
  )
  const worker = roster.value.find((item) => item.id === selected)
  if (worker)
    return (
      <WorkerConversation
        key={worker.id}
        worker={worker}
        projectId={projectId}
        sessionId={sessionId}
        disabled={disabled}
        back={() => setSelected('')}
        refreshRoster={roster.refresh}
        rosterError={roster.error}
      />
    )
  return (
    <section className="agents-panel" aria-label={t('Агенты')}>
      <header className="agents-header">
        <div className="agents-heading">
          <h3>{t('Агенты')}</h3>
          <span className="agents-count">{roster.value.length}</span>
          <button
            className="icon small"
            title={t('Обновить')}
            aria-label={t('Обновить')}
            onClick={roster.refresh}
          >
            <RefreshCw size={14} />
          </button>
        </div>
        <p>{t('Команда текущего чата. Выберите агента, чтобы открыть его переписку.')}</p>
        {disabled && (
          <p className="agents-notice">{t('Запуск новых агентов отключён в настройках.')}</p>
        )}
      </header>
      {roster.error && (
        <div className="agent-error" role="alert">
          <span>{roster.error}</span>
          <button className="quiet" onClick={roster.refresh}>
            {t('Повторить')}
          </button>
        </div>
      )}
      {roster.loading ? (
        <p className="agents-loading" role="status">
          {t('Загрузка…')}
        </p>
      ) : !roster.value.length && !roster.error ? (
        <div className="agents-empty">
          <span className="agents-empty-icon">
            <Users size={20} />
          </span>
          <strong>{t('В этом чате пока нет агентов')}</strong>
          <p>{t('Для больших задач главный агент запускает исполнителей параллельно.')}</p>
        </div>
      ) : null}
      <div className="agent-roster">
        {roster.value.map((item) => (
          <button key={item.id} className="agent-worker" onClick={() => setSelected(item.id)}>
            <div className="agent-worker-heading">
              <span className={`agent-dot agent-dot-${item.status}`} aria-hidden="true" />
              <strong>{item.name || item.agent}</strong>
              <ChevronRight size={14} />
            </div>
            <div className="agent-worker-meta">
              <span className="agent-role">{item.agent}</span>
              <span className={`agent-status agent-status-${item.status}`}>
                {t(workerStatusLabels[item.status] || item.status)}
              </span>
            </div>
            {item.task && <span className="agent-task">{item.task}</span>}
          </button>
        ))}
      </div>
    </section>
  )
}
