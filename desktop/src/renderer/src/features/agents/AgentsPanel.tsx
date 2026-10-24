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
      <div className="panel-title">
        <span>
          {t('Агенты')} · {roster.value.length}
        </span>
        <button
          className="icon small"
          title={t('Обновить')}
          aria-label={t('Обновить')}
          onClick={roster.refresh}
        >
          <RefreshCw size={14} />
        </button>
      </div>
      <p className="panel-help">
        {t('Команда текущего чата. Выберите агента, чтобы открыть его переписку.')}
      </p>
      {disabled && <p className="panel-help">{t('Запуск новых агентов отключён в настройках.')}</p>}
      {roster.error && (
        <div className="agent-error" role="alert">
          {roster.error}
          <button className="quiet" onClick={roster.refresh}>
            {t('Повторить')}
          </button>
        </div>
      )}
      {roster.loading ? (
        <p className="panel-help" role="status">
          {t('Загрузка…')}
        </p>
      ) : !roster.value.length && !roster.error ? (
        <div className="panel-empty">
          <Users size={28} />
          <p>{t('В этом чате пока нет агентов')}</p>
        </div>
      ) : null}
      <div className="agent-roster">
        {roster.value.map((item) => (
          <button key={item.id} className="agent-worker" onClick={() => setSelected(item.id)}>
            <div className="agent-worker-heading">
              <strong>{item.name || item.agent}</strong>
              <ChevronRight size={14} />
            </div>
            <span className={`agent-status agent-status-${item.status}`}>
              {t(workerStatusLabels[item.status] || item.status)}
            </span>
            <span className="agent-task">{item.task}</span>
          </button>
        ))}
      </div>
    </section>
  )
}
