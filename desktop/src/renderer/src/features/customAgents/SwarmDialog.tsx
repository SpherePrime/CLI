import { useMemo, useState } from 'react'
import { CheckCircle2, Play, TriangleAlert, X } from 'lucide-react'
import type {
  CustomAgent,
  SwarmRequest,
  SwarmResult
} from '../../../../shared/customAgents'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import { agentRoleLabel, runStatusMeta } from './presentation'

export function SwarmDialog({
  agents,
  selectedAgent,
  lastResult,
  onDispatchSwarm,
  onShowBoard,
  close
}: {
  agents: CustomAgent[]
  selectedAgent: CustomAgent | null
  lastResult: SwarmResult | null
  onDispatchSwarm: (input: SwarmRequest) => Promise<SwarmResult>
  onShowBoard?: (chainId: string) => void
  close: () => void
}) {
  const { t } = useI18n()
  const [goal, setGoal] = useState('')
  const [criteria, setCriteria] = useState('')
  const [tasks, setTasks] = useState<Record<string, string>>(() => {
    const initialId =
      selectedAgent && selectedAgent.role !== 'verifier'
        ? selectedAgent.id
        : agents.find((item) => item.role !== 'verifier')?.id ||
          selectedAgent?.id ||
          agents[0]?.id ||
          ''
    return initialId ? { [initialId]: '' } : {}
  })
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<SwarmResult | null>(lastResult)

  const selectedWorkers = useMemo(
    () => agents.filter((item) => Object.prototype.hasOwnProperty.call(tasks, item.id)),
    [agents, tasks]
  )

  const toggleWorker = (agentId: string) => {
    setTasks((current) => {
      if (Object.prototype.hasOwnProperty.call(current, agentId)) {
        const next = { ...current }
        delete next[agentId]
        return next
      }
      return { ...current, [agentId]: '' }
    })
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    const trimmedGoal = goal.trim()
    if (!trimmedGoal || selectedWorkers.length === 0 || running) return
    setRunning(true)
    setError('')
    try {
      const next = await onDispatchSwarm({
        goal: trimmedGoal,
        acceptance_criteria: criteria.trim() || undefined,
        workers: selectedWorkers.map((item) => {
          const task = tasks[item.id]?.trim()
          return task ? { agent_id: item.id, task } : { agent_id: item.id }
        })
      })
      setResult(next)
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setRunning(false)
    }
  }

  return (
    <Modal
      title={t('Запустить рой (Swarm)')}
      close={() => {
        if (!running) close()
      }}
      wide
      className="ca-swarm-modal"
    >
      <div className="ca-swarm-modal-body">
        <form className="ca-swarm-form" onSubmit={handleSubmit}>
          <label className="ca-field">
            <span>{t('Общая цель')}</span>
            <textarea
              rows={2}
              value={goal}
              onChange={(event) => setGoal(event.target.value)}
              placeholder={t('Опишите цель для параллельного выполнения агентами…')}
            />
          </label>

          <div className="ca-field">
            <span>{t('Участники роя (1+)')}</span>
            <div className="ca-swarm-picker" role="group" aria-label={t('Выбор агентов роя')}>
              {agents.map((item) => {
                const active = Object.prototype.hasOwnProperty.call(tasks, item.id)
                return (
                  <button
                    key={item.id}
                    type="button"
                    className={`ca-pill-btn ${active ? 'active' : ''}`}
                    aria-pressed={active}
                    onClick={() => toggleWorker(item.id)}
                  >
                    {item.name}
                  </button>
                )
              })}
            </div>
          </div>

          {selectedWorkers.length > 0 && (
            <div className="ca-swarm-workers">
              {selectedWorkers.map((item) => (
                <div key={item.id} className="ca-swarm-worker-row">
                  <div className="ca-swarm-worker-meta">
                    <strong>{item.name}</strong>
                    <span className="ca-chip ca-chip-muted">
                      {t(agentRoleLabel(item.role))}
                    </span>
                  </div>
                  <input
                    type="text"
                    value={tasks[item.id] ?? ''}
                    onChange={(event) =>
                      setTasks((current) => ({
                        ...current,
                        [item.id]: event.target.value
                      }))
                    }
                    placeholder={t('Подзадача для {name} (необязательно)', {
                      name: item.name
                    })}
                    aria-label={t('Подзадача для {name}', { name: item.name })}
                  />
                  <button
                    type="button"
                    className="icon small"
                    aria-label={t('Убрать {name}', { name: item.name })}
                    onClick={() => toggleWorker(item.id)}
                  >
                    <X size={13} aria-hidden="true" />
                  </button>
                </div>
              ))}
            </div>
          )}

          <label className="ca-field">
            <span>{t('Критерии приёмки (необязательно)')}</span>
            <input
              type="text"
              value={criteria}
              onChange={(event) => setCriteria(event.target.value)}
              placeholder={t('Условия прохождения верификатора роя')}
            />
          </label>

          {error && (
            <div className="ca-error-inline" role="alert">
              {error}
            </div>
          )}

          <div className="ca-swarm-actions">
            <span className="ca-muted-note">
              {selectedWorkers.length === 0
                ? t('Выберите минимум одного агента')
                : t('Параллельный запуск с авто-проверкой и синтезом')}
            </span>
            <div className="ca-modal-actions">
              <button type="button" onClick={close} disabled={running}>
                {t('Закрыть')}
              </button>
              <button
                type="submit"
                className="primary"
                disabled={running || !goal.trim() || selectedWorkers.length === 0}
              >
                <Play size={14} aria-hidden="true" />
                <span>{running ? t('Выполняется…') : t('Запустить рой')}</span>
              </button>
            </div>
          </div>
        </form>

        {result && (
          <div className="ca-swarm-result">
            <div className="ca-run-head">
              <div className="ca-run-head-left">
                <span
                  className={`ca-status-badge tone-${runStatusMeta(result.status).tone}`}
                >
                  <span className="ca-status-dot" aria-hidden="true" />
                  {t(runStatusMeta(result.status).label)}
                </span>
                <span className="ca-chip ca-chip-muted">
                  {t('Попыток: {count}', { count: result.attempts || 1 })}
                </span>
              </div>
              <div className="ca-run-head-right">
                <code>{result.chain_id}</code>
                {onShowBoard && result.chain_id && (
                  <button
                    type="button"
                    className="ca-mini-btn"
                    onClick={() => {
                      onShowBoard(result.chain_id)
                      close()
                    }}
                  >
                    {t('Открыть доску')}
                  </button>
                )}
              </div>
            </div>

            {result.verifier_verdict && (
              <div
                className={`ca-verdict-banner ${result.verifier_verdict === 'PASS' ? 'pass' : 'fail'}`}
              >
                <div className="ca-verdict-head">
                  {result.verifier_verdict === 'PASS' ? (
                    <CheckCircle2 size={15} aria-hidden="true" />
                  ) : (
                    <TriangleAlert size={15} aria-hidden="true" />
                  )}
                  <strong>{`VERDICT: ${result.verifier_verdict}`}</strong>
                </div>
                {result.verifier_feedback && (
                  <pre className="ca-verdict-text">{result.verifier_feedback}</pre>
                )}
              </div>
            )}

            {result.workers && result.workers.length > 0 && (
              <div className="ca-block">
                <span className="ca-block-label">{t('Воркеры роя')}</span>
                <div className="ca-swarm-results-grid">
                  {result.workers.map((worker, index) => {
                    const wMeta = runStatusMeta(worker.status)
                    return (
                      <article
                        key={`${worker.agent_id}-${index}`}
                        className="ca-swarm-worker-card"
                      >
                        <div className="ca-swarm-worker-card-head">
                          <strong>{worker.name || worker.agent_id}</strong>
                          <span className={`ca-status-badge tone-${wMeta.tone}`}>
                            <span className="ca-status-dot" aria-hidden="true" />
                            {t(wMeta.label)}
                          </span>
                        </div>
                        {worker.task && <p className="ca-muted-note">{worker.task}</p>}
                        {worker.error ? (
                          <p className="ca-error-inline">{worker.error}</p>
                        ) : worker.output ? (
                          <pre className="ca-output-box">{worker.output}</pre>
                        ) : null}
                      </article>
                    )
                  })}
                </div>
              </div>
            )}

            {result.final_output && (
              <div className="ca-block">
                <span className="ca-block-label">{t('Итоговый синтез')}</span>
                <pre className="ca-output-box">{result.final_output}</pre>
              </div>
            )}

            {result.error && (
              <div className="ca-error-inline" role="alert">
                {result.error}
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  )
}
