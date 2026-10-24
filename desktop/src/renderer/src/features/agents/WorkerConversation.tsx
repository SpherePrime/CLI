import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { ArrowLeft, RefreshCw, Send, Square } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { workerActive, workerStatusLabels, type AgentWorker } from '../../../../shared/agents'
import { useI18n } from '../../i18n'
import { MessageList } from '../chat/MessageList'
import { reuseUnchanged } from '../chat/stableMessages'
import { useAgentPolling } from './useAgentPolling'

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
  const { t } = useI18n()
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
  const snapshot = useRef<Message[]>([])
  const revision = useRef('')
  const current = useRef(worker)
  current.current = worker
  const load = useCallback(async () => {
    const nextRevision = `${current.current.status}:${current.current.updated_at}`
    if (!workerActive(current.current) && revision.current === nextRevision) return snapshot.current
    const result = await window.prime.call(projectId, 'agentWorkerMessages', {
      sessionId,
      workerId: worker.id
    })
    revision.current = nextRevision
    snapshot.current = reuseUnchanged(snapshot.current, result.messages || [])
    return snapshot.current
  }, [projectId, sessionId, worker.id])
  const history = useAgentPolling(load, workerActive(worker) ? 1000 : 3000, [] as Message[])
  function refresh() {
    revision.current = ''
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
  return (
    <section className="agents-panel agent-conversation" aria-label={worker.name || worker.agent}>
      <header className="agent-chat-header">
        <button
          className="icon small"
          aria-label={t('К списку агентов')}
          title={t('К списку агентов')}
          onClick={back}
        >
          <ArrowLeft size={16} />
        </button>
        <div className="agent-chat-name">
          <strong>{worker.name || worker.agent}</strong>
          <div className="agent-worker-meta">
            <span className={`agent-dot agent-dot-${worker.status}`} aria-hidden="true" />
            <span className={`agent-status agent-status-${worker.status}`}>
              {t(workerStatusLabels[worker.status] || worker.status)}
            </span>
            <span className="agent-role">{worker.agent}</span>
          </div>
        </div>
        <button
          className="icon small"
          aria-label={t('Обновить')}
          title={t('Обновить')}
          onClick={refresh}
        >
          <RefreshCw size={14} />
        </button>
        {workerActive(worker) && (
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
      </header>
      <details className="agent-assignment">
        <summary>{t('Задание агента')}</summary>
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
      {history.loading ? (
        <p className="agents-loading" role="status">
          {t('Загрузка…')}
        </p>
      ) : history.value.length ? (
        <MessageList messages={history.value} busy={worker.status === 'running'} />
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
          rows={3}
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
