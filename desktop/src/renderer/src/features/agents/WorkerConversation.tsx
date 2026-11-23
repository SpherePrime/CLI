import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ArrowLeft, ChevronDown, Clock, RefreshCw, Send, Square } from 'lucide-react'
import { workerActive, workerStatusLabels, type AgentWorker } from '../../../../shared/agents'
import { useI18n } from '../../i18n'
import { MessageList } from '../chat/MessageList'
import { WorkspaceMediaProvider } from '../chat/WorkspaceMediaContext'
import { WorkerRoleIcon } from './AgentsPanel'
import { formatWorkerDuration, workerRoleKind } from './agentPresentation'
import { useWorkerMessageHistory } from './useWorkerMessageHistory'

export function WorkerConversation({
  worker,
  projectId,
  sessionId,
  disabled,
  back,
  refreshRoster,
  rosterError
}: {
  worker: AgentWorker
  projectId: string
  sessionId: string
  disabled: boolean
  back: () => void
  refreshRoster: () => void
  rosterError: string
}) {
  const { t, language } = useI18n()
  const [draft, setDraft] = useState('')
  const [pending, setPending] = useState(false)
  const [actionError, setActionError] = useState('')
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const active = workerActive(worker)
  const [nowSeconds, setNowSeconds] = useState(() => Math.floor(Date.now() / 1000))
  useEffect(() => {
    if (!active) return
    setNowSeconds(Math.floor(Date.now() / 1000))
    const timer = window.setInterval(() => setNowSeconds(Math.floor(Date.now() / 1000)), 1000)
    return () => window.clearInterval(timer)
  }, [active])
  const history = useWorkerMessageHistory({ projectId, sessionId, worker })
  function refresh() {
    history.refresh()
    refreshRoster()
  }
  async function act(stop: boolean) {
    if (pending || (!stop && !draft.trim())) return
    setPending(true)
    setActionError('')
    try {
      await window.prime.call(projectId, stop ? 'agentWorkerStop' : 'agentWorkerSend', {
        sessionId,
        workerId: worker.id,
        ...(!stop ? { text: draft.trim() } : {})
      })
      if (!mounted.current) return
      if (!stop) setDraft('')
      refresh()
    } catch (error) {
      if (mounted.current) setActionError(error instanceof Error ? error.message : String(error))
    } finally {
      if (mounted.current) setPending(false)
    }
  }
  const submit = (event: FormEvent) => {
    event.preventDefault()
    void act(false)
  }
  const roleKind = workerRoleKind(worker.agent)
  const duration = formatWorkerDuration(
    worker.started_at,
    worker.updated_at,
    active,
    language,
    nowSeconds
  )
  return (
    <section className="agents-panel agent-conversation" aria-label={worker.name || worker.agent}>
      <header className="agent-chat-header">
        <div className="agent-chat-header-inner">
          <button
            className="icon small"
            aria-label={t('К списку агентов')}
            title={t('К списку агентов')}
            onClick={back}
          >
            <ArrowLeft size={16} />
          </button>
          <span className={`agent-role-icon role-${roleKind}`}>
            <WorkerRoleIcon role={worker.agent} size={15} />
          </span>
          <div className="agent-chat-name">
            <strong>{worker.name || worker.agent}</strong>
            <div className="agent-worker-meta">
              <span className={`agent-status-pill agent-status-${worker.status}`}>
                <span className={`agent-dot agent-dot-${worker.status}`} aria-hidden="true" />
                {t(workerStatusLabels[worker.status] || worker.status)}
              </span>
              <span className="agent-role">{worker.agent}</span>
              {duration && (
                <span className="agent-duration">
                  <Clock size={11} aria-hidden="true" />
                  <span>{duration}</span>
                </span>
              )}
            </div>
          </div>
          <div className="agent-chat-actions">
            <button
              className="icon small"
              aria-label={t('Обновить')}
              title={t('Обновить')}
              onClick={refresh}
            >
              <RefreshCw size={14} />
            </button>
            {active && (
              <button
                className="icon small agent-stop"
                disabled={pending}
                aria-label={t('Остановить агента')}
                title={t('Остановить агента')}
                onClick={() => void act(true)}
              >
                <Square size={14} />
              </button>
            )}
          </div>
        </div>
      </header>
      <details className="agent-assignment">
        <summary>
          <ChevronDown size={14} className="agent-assignment-chevron" aria-hidden="true" />
          <span>{t('Задание агента')}</span>
        </summary>
        <p>{worker.task}</p>
      </details>
      {(actionError || history.error || rosterError) && (
        <div className="agent-error" role="alert">
          <span>{actionError || history.error || rosterError}</span>
          <button className="quiet" onClick={refresh}>
            {t('Повторить')}
          </button>
        </div>
      )}
      {history.initialLoading ? (
        <p className="agents-loading" role="status">
          {t('Загрузка…')}
        </p>
      ) : history.messages.length ? (
        <WorkspaceMediaProvider workspaceId={projectId}>
          <MessageList
            messages={history.messages}
            busy={worker.status === 'running'}
            hasBefore={history.hasBefore}
            hasAfter={history.hasAfter}
            historyLoading={Boolean(history.pagingLoading)}
            historyError={history.pagingError}
            historyTiming={history.timing}
            loadHistory={history.loadHistory}
          />
        </WorkspaceMediaProvider>
      ) : (
        <div className="agents-empty">
          <p>{t('Сообщений пока нет')}</p>
        </div>
      )}
      <form className="agent-composer" onSubmit={submit}>
        <label className="sr-only" htmlFor={`agent-followup-${worker.id}`}>
          {t('Сообщение агенту')}
        </label>
        <textarea
          id={`agent-followup-${worker.id}`}
          rows={2}
          value={draft}
          disabled={disabled || pending}
          onChange={(event) => setDraft(event.target.value)}
          placeholder={t('Дополнительное задание…')}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
              event.preventDefault()
              void act(false)
            }
          }}
        />
        <div className="agent-composer-footer">
          <span className="muted">
            {disabled
              ? t('Запуск новых агентов отключён в настройках.')
              : t('Разрешения и подтверждения — в основном чате.')}
          </span>
          <button
            className="agent-send"
            type="submit"
            aria-label={t('Отправить')}
            title={t('Отправить')}
            disabled={disabled || pending || !draft.trim()}
          >
            <Send size={14} />
          </button>
        </div>
      </form>
    </section>
  )
}
