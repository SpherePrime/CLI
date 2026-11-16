import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowLeft, ChevronRight, LoaderCircle, RefreshCw, Users } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { AgentWorker } from '../../../../shared/agents'
import { workerStatusLabels } from '../../../../shared/agents'
import { useI18n } from '../../i18n'
import { useAgentPolling } from './useAgentPolling'
import { WorkerConversation } from './WorkerConversation'
import {
  agentRosterPollInterval,
  scopeAgentRequest,
  type AgentRequest
} from './openAgent'
import { realSessionId } from '../../../../shared/pendingSession'
import './agents.css'

export function AgentsPanel({
  workspace,
  request,
  onConsumeAgentRequest
}: {
  workspace: Workspace
  request?: AgentRequest
  onConsumeAgentRequest?: (sequence: number) => void
}) {
  const { t } = useI18n()
  const sessionId = realSessionId(workspace.session)
  if (!workspace.project || !sessionId)
    return (
      <div className="panel-empty">
        <Users size={28} />
        <p>{t('Откройте чат, чтобы увидеть его агентов')}</p>
      </div>
    )
  const scopedRequest = scopeAgentRequest(request, sessionId)
  return (
    <AgentTeam
      key={`${workspace.project.id}:${sessionId}`}
      projectId={workspace.project.id}
      sessionId={sessionId}
      disabled={!!workspace.config.options?.disable_multi_agent}
      request={scopedRequest}
      onConsumeAgentRequest={onConsumeAgentRequest}
    />
  )
}

function AgentTeam({
  projectId,
  sessionId,
  disabled,
  request,
  onConsumeAgentRequest
}: {
  projectId: string
  sessionId: string
  disabled: boolean
  request?: AgentRequest
  onConsumeAgentRequest?: (sequence: number) => void
}) {
  const { t } = useI18n()
  const [selected, setSelected] = useState(() => request?.workerId || '')
  const selectedRef = useRef(selected)
  selectedRef.current = selected
  useEffect(() => {
    if (!request?.workerId) return
    setSelected(request.workerId)
    onConsumeAgentRequest?.(request.sequence)
  }, [request?.sequence, request?.workerId, onConsumeAgentRequest])
  const load = useCallback(async () => {
    const result = await window.prime.call(projectId, 'agentWorkers', { sessionId })
    const workers = (result.workers || []) as AgentWorker[]
    return workers
  }, [projectId, sessionId])
  const roster = useAgentPolling(
    load,
    (workers) => agentRosterPollInterval(workers, selectedRef.current),
    [] as AgentWorker[]
  )
  useEffect(() => {
    if (selected && !roster.value.some((item) => item.id === selected)) roster.refresh()
  }, [selected])
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
  if (selected)
    return (
      <section className="agents-panel agent-conversation" aria-label={t('Агент')}>
        <header className="agent-chat-header">
          <button
            className="icon small"
            aria-label={t('К списку агентов')}
            title={t('К списку агентов')}
            onClick={() => setSelected('')}
          >
            <ArrowLeft size={16} />
          </button>
          <div className="agent-chat-name">
            <strong>{t('Агент')}</strong>
          </div>
          <button
            className="icon small"
            aria-label={t('Обновить')}
            title={t('Обновить')}
            onClick={roster.refresh}
          >
            <RefreshCw size={14} />
          </button>
        </header>
        {roster.error && (
          <div className="agent-error" role="alert">
            <span>{roster.error}</span>
            <button className="quiet" onClick={roster.refresh}>
              {t('Повторить')}
            </button>
          </div>
        )}
        <div className="agents-empty" role="status">
          <LoaderCircle size={20} className="spin" />
          <span>{t('Запуск агента…')}</span>
        </div>
      </section>
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
          <p>{t('Для больших задач главный агент запускает агентов параллельно.')}</p>
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
