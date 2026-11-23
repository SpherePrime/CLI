import { useCallback, useEffect, useRef, useState } from 'react'
import {
  ArrowLeft,
  Bot,
  ChevronRight,
  Clock,
  Code2,
  Compass,
  ListChecks,
  LoaderCircle,
  RefreshCw,
  ShieldCheck,
  Users
} from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { workerActive, workerStatusLabels, type AgentWorker } from '../../../../shared/agents'
import { useI18n } from '../../i18n'
import { useAgentPolling } from './useAgentPolling'
import { WorkerConversation } from './WorkerConversation'
import {
  activeWorkerCount,
  formatWorkerDuration,
  workerRoleKind,
  type WorkerRoleKind
} from './agentPresentation'
import {
  agentRosterPollInterval,
  scopeAgentRequest,
  type AgentRequest
} from './openAgent'
import { realSessionId } from '../../../../shared/pendingSession'
import './agents.css'

export function WorkerRoleIcon({ role, size = 15 }: { role: string; size?: number }) {
  const kind: WorkerRoleKind = workerRoleKind(role)
  if (kind === 'code') return <Code2 size={size} aria-hidden="true" />
  if (kind === 'explore') return <Compass size={size} aria-hidden="true" />
  if (kind === 'plan') return <ListChecks size={size} aria-hidden="true" />
  if (kind === 'review') return <ShieldCheck size={size} aria-hidden="true" />
  return <Bot size={size} aria-hidden="true" />
}

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
  const { t, language } = useI18n()
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
  const activeCount = activeWorkerCount(roster.value)
  const [nowSeconds, setNowSeconds] = useState(() => Math.floor(Date.now() / 1000))
  useEffect(() => {
    if (activeCount === 0) return
    setNowSeconds(Math.floor(Date.now() / 1000))
    const timer = window.setInterval(() => setNowSeconds(Math.floor(Date.now() / 1000)), 1000)
    return () => window.clearInterval(timer)
  }, [activeCount])
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
          <div className="agent-chat-header-inner">
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
          </div>
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
        <div className="agents-header-inner">
          <div className="agents-heading">
            <h3>{t('Агенты')}</h3>
            <span className="agents-count">{roster.value.length}</span>
            {activeCount > 0 && (
              <span className="agents-active-pill">
                <span className="agent-dot agent-dot-running" aria-hidden="true" />
                {t('{count} в работе', { count: activeCount })}
              </span>
            )}
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
        </div>
      </header>
      <div className="agents-scroll">
        <div className="agents-body">
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
          <div className="agent-roster" data-count={roster.value.length}>
            {roster.value.map((item) => {
              const roleKind = workerRoleKind(item.agent)
              const active = workerActive(item)
              const duration = formatWorkerDuration(
                item.started_at,
                item.updated_at,
                active,
                language,
                nowSeconds
              )
              return (
                <button
                  key={item.id}
                  type="button"
                  className="agent-worker"
                  data-status={item.status}
                  data-role={roleKind}
                  onClick={() => setSelected(item.id)}
                >
                  <div className="agent-worker-top">
                    <span className={`agent-role-icon role-${roleKind}`}>
                      <WorkerRoleIcon role={item.agent} size={15} />
                    </span>
                    <div className="agent-worker-title-group">
                      <strong>{item.name || item.agent}</strong>
                    </div>
                    {duration && (
                      <span className="agent-duration" title={duration}>
                        <Clock size={11} aria-hidden="true" />
                        <span>{duration}</span>
                      </span>
                    )}
                    <ChevronRight size={15} className="agent-worker-chevron" aria-hidden="true" />
                  </div>
                  <div className="agent-worker-badges">
                    <span className={`agent-status-pill agent-status-${item.status}`}>
                      <span className={`agent-dot agent-dot-${item.status}`} aria-hidden="true" />
                      {t(workerStatusLabels[item.status] || item.status)}
                    </span>
                    <span className="agent-role">{item.agent}</span>
                  </div>
                  {item.task && <span className="agent-task">{item.task}</span>}
                </button>
              )
            })}
          </div>
        </div>
      </div>
    </section>
  )
}
