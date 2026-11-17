import { useEffect, useMemo, useState } from 'react'
import {
  Activity,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  GitBranch,
  History,
  Layers,
  MessageSquarePlus,
  Play,
  Radar,
  Send,
  Square,
  TriangleAlert,
  X
} from 'lucide-react'
import type {
  BlackboardEntry,
  CustomAgent,
  CustomAgentRun,
  ProbeResult,
  SwarmRequest,
  SwarmResult
} from '../../../../shared/customAgents'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import {
  agentRoleLabel,
  agentStatusMeta,
  formatDuration,
  formatTimestamp,
  isActiveRunStatus,
  ORCHESTRATOR_QUICK_PROMPTS,
  runStatusMeta,
  triggerLabel
} from './presentation'

type CenterTab = 'tasks' | 'chain' | 'history'

export function AgentWorkspace({
  agent,
  agents,
  runs,
  allRuns,
  blackboard,
  selectedChainId,
  onSelectChainId,
  onSelectAgent,
  onRun,
  onCancelRun,
  onProbe,
  onPostBlackboard,
  onDispatchSwarm
}: {
  agent: CustomAgent
  agents: CustomAgent[]
  runs: CustomAgentRun[]
  allRuns: CustomAgentRun[]
  blackboard: BlackboardEntry[]
  selectedChainId: string
  onSelectChainId: (chainId: string) => void
  onSelectAgent: (agentId: string) => void
  onRun: (params: {
    agentId: string
    prompt: string
    triggerReason?: string
    chainId?: string
    force?: boolean
  }) => Promise<CustomAgentRun>
  onCancelRun: (agentId: string, runId: string) => Promise<unknown>
  onProbe: (agentId: string) => Promise<ProbeResult>
  onPostBlackboard: (input: {
    chain_id: string
    from_agent_id?: string
    from_agent_name?: string
    to_agent_id?: string
    topic: string
    payload: string
  }) => Promise<BlackboardEntry>
  onDispatchSwarm: (input: SwarmRequest) => Promise<SwarmResult>
}) {
  const { t, locale } = useI18n()
  const [tab, setTab] = useState<CenterTab>('tasks')
  const [prompt, setPrompt] = useState('')
  const [forceRun, setForceRun] = useState(false)
  const [dispatching, setDispatching] = useState(false)
  const [probing, setProbing] = useState(false)
  const [probeResult, setProbeResult] = useState<ProbeResult | null>(null)
  const [historyScope, setHistoryScope] = useState<'agent' | 'all'>('agent')
  const [boardChain, setBoardChain] = useState(selectedChainId || '')
  const [boardTo, setBoardTo] = useState('')
  const [boardTopic, setBoardTopic] = useState('note')
  const [boardPayload, setBoardPayload] = useState('')
  const [postingBoard, setPostingBoard] = useState(false)
  const [swarmOpen, setSwarmOpen] = useState(true)
  const [swarmGoal, setSwarmGoal] = useState('')
  const [swarmCriteria, setSwarmCriteria] = useState('')
  const [swarmTasks, setSwarmTasks] = useState<Record<string, string>>(() => {
    const initialId =
      agent.role !== 'verifier'
        ? agent.id
        : agents.find((item) => item.role !== 'verifier')?.id || agent.id
    return initialId ? { [initialId]: '' } : {}
  })
  const [swarmRunning, setSwarmRunning] = useState(false)
  const [swarmError, setSwarmError] = useState('')
  const [swarmResult, setSwarmResult] = useState<SwarmResult | null>(null)
  const [actionError, setActionError] = useState('')

  useEffect(() => {
    setProbeResult(null)
    setActionError('')
  }, [agent.id])

  useEffect(() => {
    setSwarmTasks((current) => {
      const existingIds = new Set(agents.map((item) => item.id))
      const filtered: Record<string, string> = {}
      for (const [id, value] of Object.entries(current)) {
        if (existingIds.has(id)) filtered[id] = value
      }
      if (Object.keys(filtered).length > 0) return filtered
      const fallback =
        agent.role !== 'verifier'
          ? agent.id
          : agents.find((item) => item.role !== 'verifier')?.id || agent.id
      return fallback ? { [fallback]: '' } : {}
    })
  }, [agents, agent.id, agent.role])

  useEffect(() => {
    if (selectedChainId) setBoardChain(selectedChainId)
  }, [selectedChainId])

  const statusMeta = agentStatusMeta(agent.status)
  const activeRun = useMemo(() => runs.find((run) => isActiveRunStatus(run.status)) || null, [runs])
  const latestRun = runs[0] || null

  const verifierAgent = useMemo(() => {
    if (agent.verifier_agent_id) {
      return agents.find((a) => a.id === agent.verifier_agent_id) || null
    }
    return agents.find((a) => a.role === 'verifier' && a.enabled && a.id !== agent.id) || null
  }, [agents, agent.verifier_agent_id, agent.id])

  const nextAgent = useMemo(
    () => (agent.next_agent_id ? agents.find((a) => a.id === agent.next_agent_id) || null : null),
    [agents, agent.next_agent_id]
  )

  const chainIds = useMemo(() => {
    const ids = new Set<string>()
    for (const run of allRuns) {
      if (run.chain_id) ids.add(run.chain_id)
    }
    for (const entry of blackboard) {
      if (entry.chain_id) ids.add(entry.chain_id)
    }
    return [...ids]
  }, [allRuns, blackboard])

  const visibleBoard = useMemo(() => {
    if (!selectedChainId) return blackboard
    return blackboard.filter((entry) => entry.chain_id === selectedChainId)
  }, [blackboard, selectedChainId])

  const visibleHistory = historyScope === 'all' ? allRuns : runs

  const handleDispatch = async (taskPrompt: string, triggerReason = 'manual') => {
    if (dispatching) return
    setDispatching(true)
    setActionError('')
    try {
      await onRun({
        agentId: agent.id,
        prompt: taskPrompt.trim(),
        triggerReason,
        force: forceRun || undefined
      })
      if (taskPrompt === prompt) setPrompt('')
    } catch (cause) {
      setActionError(String((cause as Error)?.message || cause))
    } finally {
      setDispatching(false)
    }
  }

  const handleProbe = async () => {
    if (probing) return
    setProbing(true)
    setActionError('')
    try {
      const res = await onProbe(agent.id)
      setProbeResult(res)
    } catch (cause) {
      setActionError(String((cause as Error)?.message || cause))
    } finally {
      setProbing(false)
    }
  }

  const selectedSwarmAgents = useMemo(
    () => agents.filter((item) => Object.prototype.hasOwnProperty.call(swarmTasks, item.id)),
    [agents, swarmTasks]
  )

  const toggleSwarmAgent = (agentId: string) => {
    setSwarmTasks((current) => {
      if (Object.prototype.hasOwnProperty.call(current, agentId)) {
        const next = { ...current }
        delete next[agentId]
        return next
      }
      return { ...current, [agentId]: '' }
    })
  }

  const handleDispatchSwarm = async (event: React.FormEvent) => {
    event.preventDefault()
    const goal = swarmGoal.trim()
    if (!goal || selectedSwarmAgents.length === 0 || swarmRunning) return
    setSwarmRunning(true)
    setSwarmError('')
    try {
      const result = await onDispatchSwarm({
        goal,
        acceptance_criteria: swarmCriteria.trim() || undefined,
        workers: selectedSwarmAgents.map((item) => {
          const task = swarmTasks[item.id]?.trim()
          return task ? { agent_id: item.id, task } : { agent_id: item.id }
        })
      })
      setSwarmResult(result)
    } catch (cause) {
      setSwarmError(String((cause as Error)?.message || cause))
    } finally {
      setSwarmRunning(false)
    }
  }

  const handlePostBoard = async (event: React.FormEvent) => {
    event.preventDefault()
    const payload = boardPayload.trim()
    if (!payload || postingBoard) return
    const chainId = boardChain.trim() || latestRun?.chain_id || `chain-${Date.now()}`
    setPostingBoard(true)
    setActionError('')
    try {
      await onPostBlackboard({
        chain_id: chainId,
        from_agent_id: agent.id,
        from_agent_name: agent.name,
        to_agent_id: boardTo,
        topic: boardTopic.trim() || 'note',
        payload
      })
      setBoardPayload('')
      setBoardChain(chainId)
      onSelectChainId(chainId)
    } catch (cause) {
      setActionError(String((cause as Error)?.message || cause))
    } finally {
      setPostingBoard(false)
    }
  }

  const hasMonitor = Boolean(agent.monitor_kind && agent.monitor_target)
  const hasVerifier =
    agent.role !== 'verifier' &&
    (Boolean(agent.acceptance_criteria.trim()) ||
      (agent.deterministic_checks && agent.deterministic_checks.length > 0) ||
      Boolean(agent.verifier_agent_id))

  return (
    <section className="ca-workspace" aria-label={t('Рабочая зона агента')}>
      <header className="ca-workspace-head">
        <div className="ca-workspace-title">
          <div className="ca-workspace-title-row">
            <h2>{agent.name}</h2>
            <span className="ca-chip">{t(agentRoleLabel(agent.role))}</span>
            <span className={`ca-status-badge tone-${statusMeta.tone}`}>
              <span className="ca-status-dot" aria-hidden="true" />
              {t(statusMeta.label)}
            </span>
          </div>
          {agent.description && <p className="ca-workspace-desc">{agent.description}</p>}
        </div>

        <div className="ca-workspace-actions">
          {hasMonitor && (
            <button type="button" onClick={() => void handleProbe()} disabled={probing}>
              <Radar size={15} aria-hidden="true" />
              <span>{probing ? t('Проверяем…') : t('Проверить зонд')}</span>
            </button>
          )}
          {activeRun ? (
            <button
              type="button"
              className="danger"
              onClick={() => void onCancelRun(agent.id, activeRun.id)}
            >
              <Square size={14} aria-hidden="true" />
              <span>{t('Остановить')}</span>
            </button>
          ) : (
            <button
              type="button"
              className="primary"
              disabled={dispatching}
              onClick={() => void handleDispatch(prompt || '')}
            >
              <Play size={14} aria-hidden="true" />
              <span>{dispatching ? t('Запуск…') : t('Запустить')}</span>
            </button>
          )}
        </div>
      </header>

      <div className="ca-subtabs" role="tablist" aria-label={t('Разделы агента')}>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'tasks'}
          className={tab === 'tasks' ? 'active' : ''}
          onClick={() => setTab('tasks')}
        >
          <Activity size={14} aria-hidden="true" />
          <span>{t('Задачи и чат')}</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'chain'}
          className={tab === 'chain' ? 'active' : ''}
          onClick={() => setTab('chain')}
        >
          <GitBranch size={14} aria-hidden="true" />
          <span>{t('Цепочка и доска')}</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'history'}
          className={tab === 'history' ? 'active' : ''}
          onClick={() => setTab('history')}
        >
          <History size={14} aria-hidden="true" />
          <span>{t('История запусков')}</span>
          <span className="ca-tab-count">{runs.length}</span>
        </button>
      </div>

      {actionError && (
        <div className="ca-error-banner" role="alert">
          <TriangleAlert size={15} aria-hidden="true" />
          <span>{actionError}</span>
        </div>
      )}

      <div className="ca-workspace-body">
        {probeResult && (
          <div
            className={`ca-probe-banner ${probeResult.error ? 'tone-red' : probeResult.changed ? 'tone-amber' : 'tone-green'}`}
          >
            <div className="ca-probe-head">
              <strong>
                {probeResult.error
                  ? t('Ошибка зонда')
                  : probeResult.changed
                    ? t('Зонд: обнаружены изменения')
                    : t('Зонд: изменений нет')}
              </strong>
              <div className="ca-probe-head-right">
                {probeResult.hash && <code>{probeResult.hash.slice(0, 12)}</code>}
                <button
                  type="button"
                  className="icon small"
                  aria-label={t('Закрыть')}
                  onClick={() => setProbeResult(null)}
                >
                  <X size={14} aria-hidden="true" />
                </button>
              </div>
            </div>
            {probeResult.error ? (
              <p>{probeResult.error}</p>
            ) : probeResult.diff ? (
              <pre className="ca-code-box">{probeResult.diff}</pre>
            ) : null}
          </div>
        )}

        {tab === 'tasks' && (
          <div className="ca-tasks-pane">
            <form
              className="ca-prompt-box"
              onSubmit={(event) => {
                event.preventDefault()
                void handleDispatch(prompt)
              }}
            >
              <textarea
                rows={3}
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={
                  agent.role === 'orchestrator'
                    ? t('Поставьте задачу координатору или попросите создать нового агента…')
                    : t('Введите задачу для агента или оставьте пустым для запуска по инструкции…')
                }
                aria-label={t('Задание агенту')}
              />
              <div className="ca-prompt-bar">
                {hasMonitor ? (
                  <label className="ca-checkbox">
                    <input
                      type="checkbox"
                      checked={forceRun}
                      onChange={(event) => setForceRun(event.target.checked)}
                    />
                    <span>{t('Игнорировать кэш зонда')}</span>
                  </label>
                ) : (
                  <span className="ca-muted-note">{triggerLabel(agent)}</span>
                )}
                <button type="submit" className="primary" disabled={dispatching}>
                  <Send size={14} aria-hidden="true" />
                  <span>{dispatching ? t('Отправка…') : t('Отправить')}</span>
                </button>
              </div>
            </form>

            {agent.role === 'orchestrator' && (
              <div className="ca-quick-chips" role="group" aria-label={t('Быстрые шаблоны')}>
                {ORCHESTRATOR_QUICK_PROMPTS.map((chip) => (
                  <button
                    key={chip}
                    type="button"
                    className="ca-chip-btn"
                    onClick={() => setPrompt(chip)}
                  >
                    {t(chip)}
                  </button>
                ))}
              </div>
            )}

            {latestRun ? (
              <RunTraceCard
                run={latestRun}
                locale={locale}
                t={t}
                onCancel={() => void onCancelRun(agent.id, latestRun.id)}
              />
            ) : (
              <div className="ca-empty-card">
                <p>{t('Запусков пока не было. Отправьте задачу выше или нажмите «Запустить».')}</p>
              </div>
            )}
          </div>
        )}

        {tab === 'chain' && (
          <div className="ca-chain-pane">
            <div className="ca-pipeline" aria-label={t('Схема цепочки')}>
              <div className="ca-pipe-node">
                <span className="ca-pipe-kicker">{t('Триггер / Монитор')}</span>
                <strong>{triggerLabel(agent)}</strong>
                {agent.monitor_target && <span className="ca-pipe-sub">{agent.monitor_target}</span>}
              </div>
              <ArrowRight className="ca-pipe-arrow" size={16} aria-hidden="true" />
              <div className="ca-pipe-node current">
                <span className="ca-pipe-kicker">{t(agentRoleLabel(agent.role))}</span>
                <strong>{agent.name}</strong>
                <span className="ca-pipe-sub">
                  {agent.model_mode === 'manual' && agent.model_id ? agent.model_id : t('Авто-подбор модели')}
                </span>
              </div>
              <ArrowRight className="ca-pipe-arrow" size={16} aria-hidden="true" />
              <div className="ca-pipe-node">
                <span className="ca-pipe-kicker">{t('Верификатор')}</span>
                <strong>
                  {!hasVerifier
                    ? t('Без проверки')
                    : verifierAgent?.name || t('Детерминированные проверки')}
                </strong>
                {hasVerifier && (
                  <span className="ca-pipe-sub">
                    {t('Доработок: до {count}', { count: agent.max_rework_attempts || 3 })}
                  </span>
                )}
              </div>
              <ArrowRight className="ca-pipe-arrow" size={16} aria-hidden="true" />
              {nextAgent ? (
                <button
                  type="button"
                  className="ca-pipe-node clickable"
                  onClick={() => onSelectAgent(nextAgent.id)}
                >
                  <span className="ca-pipe-kicker">{t('Следующий агент')}</span>
                  <strong>{nextAgent.name}</strong>
                  <span className="ca-pipe-sub">{t(agentRoleLabel(nextAgent.role))}</span>
                </button>
              ) : (
                <div className="ca-pipe-node">
                  <span className="ca-pipe-kicker">{t('Передача')}</span>
                  <strong>{t('Конец цепочки')}</strong>
                </div>
              )}
            </div>

            <section className="ca-swarm-card" aria-label={t('Запустить рой (Swarm)')}>
              <div className="ca-swarm-head">
                <button
                  type="button"
                  className="ca-swarm-toggle"
                  aria-expanded={swarmOpen}
                  onClick={() => setSwarmOpen((prev) => !prev)}
                >
                  <Layers size={15} aria-hidden="true" />
                  <strong>{t('Запустить рой (Swarm)')}</strong>
                  <span className="ca-chip ca-chip-muted">
                    {t('Агентов: {count}', { count: selectedSwarmAgents.length })}
                  </span>
                  {swarmOpen ? (
                    <ChevronUp size={15} aria-hidden="true" />
                  ) : (
                    <ChevronDown size={15} aria-hidden="true" />
                  )}
                </button>
              </div>

              {swarmOpen && (
                <form className="ca-swarm-form" onSubmit={handleDispatchSwarm}>
                  <label className="ca-field">
                    <span>{t('Общая цель')}</span>
                    <textarea
                      rows={2}
                      value={swarmGoal}
                      onChange={(event) => setSwarmGoal(event.target.value)}
                      placeholder={t('Опишите цель для параллельного выполнения агентами…')}
                    />
                  </label>

                  <div className="ca-field">
                    <span>{t('Участники роя (1+)')}</span>
                    <div className="ca-swarm-picker" role="group" aria-label={t('Выбор агентов роя')}>
                      {agents.map((item) => {
                        const active = Object.prototype.hasOwnProperty.call(swarmTasks, item.id)
                        return (
                          <button
                            key={item.id}
                            type="button"
                            className={`ca-pill-btn ${active ? 'active' : ''}`}
                            aria-pressed={active}
                            onClick={() => toggleSwarmAgent(item.id)}
                          >
                            {item.name}
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  {selectedSwarmAgents.length > 0 && (
                    <div className="ca-swarm-workers">
                      {selectedSwarmAgents.map((item) => (
                        <div key={item.id} className="ca-swarm-worker-row">
                          <div className="ca-swarm-worker-meta">
                            <strong>{item.name}</strong>
                            <span className="ca-chip ca-chip-muted">
                              {t(agentRoleLabel(item.role))}
                            </span>
                          </div>
                          <input
                            type="text"
                            value={swarmTasks[item.id] ?? ''}
                            onChange={(event) =>
                              setSwarmTasks((current) => ({
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
                            onClick={() => toggleSwarmAgent(item.id)}
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
                      value={swarmCriteria}
                      onChange={(event) => setSwarmCriteria(event.target.value)}
                      placeholder={t('Условия прохождения верификатора роя')}
                    />
                  </label>

                  {swarmError && (
                    <div className="ca-error-inline" role="alert">
                      {swarmError}
                    </div>
                  )}

                  <div className="ca-swarm-actions">
                    <span className="ca-muted-note">
                      {selectedSwarmAgents.length === 0
                        ? t('Выберите минимум одного агента')
                        : t('Параллельный запуск с авто-проверкой и синтезом')}
                    </span>
                    <button
                      type="submit"
                      className="primary"
                      disabled={swarmRunning || !swarmGoal.trim() || selectedSwarmAgents.length === 0}
                    >
                      <Play size={14} aria-hidden="true" />
                      <span>{swarmRunning ? t('Выполняется…') : t('Запустить рой')}</span>
                    </button>
                  </div>
                </form>
              )}

              {swarmResult && (
                <div className="ca-swarm-result">
                  <div className="ca-run-head">
                    <div className="ca-run-head-left">
                      <span
                        className={`ca-status-badge tone-${runStatusMeta(swarmResult.status).tone}`}
                      >
                        <span className="ca-status-dot" aria-hidden="true" />
                        {t(runStatusMeta(swarmResult.status).label)}
                      </span>
                      <span className="ca-chip ca-chip-muted">
                        {t('Попыток: {count}', { count: swarmResult.attempts || 1 })}
                      </span>
                    </div>
                    <code>{swarmResult.chain_id}</code>
                  </div>

                  {swarmResult.verifier_verdict && (
                    <div
                      className={`ca-verdict-banner ${swarmResult.verifier_verdict === 'PASS' ? 'pass' : 'fail'}`}
                    >
                      <div className="ca-verdict-head">
                        {swarmResult.verifier_verdict === 'PASS' ? (
                          <CheckCircle2 size={15} aria-hidden="true" />
                        ) : (
                          <TriangleAlert size={15} aria-hidden="true" />
                        )}
                        <strong>{`VERDICT: ${swarmResult.verifier_verdict}`}</strong>
                      </div>
                      {swarmResult.verifier_feedback && (
                        <pre className="ca-verdict-text">{swarmResult.verifier_feedback}</pre>
                      )}
                    </div>
                  )}

                  {swarmResult.workers && swarmResult.workers.length > 0 && (
                    <div className="ca-block">
                      <span className="ca-block-label">{t('Воркеры роя')}</span>
                      <div className="ca-swarm-results-grid">
                        {swarmResult.workers.map((worker, index) => {
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

                  {swarmResult.final_output && (
                    <div className="ca-block">
                      <span className="ca-block-label">{t('Итоговый синтез')}</span>
                      <pre className="ca-output-box">{swarmResult.final_output}</pre>
                    </div>
                  )}

                  {swarmResult.error && (
                    <div className="ca-error-inline" role="alert">
                      {swarmResult.error}
                    </div>
                  )}
                </div>
              )}
            </section>

            <div className="ca-board-section">
              <div className="ca-board-head">
                <h3>{t('Общая доска (Blackboard)')}</h3>
                <div className="ca-board-filter">
                  <Select
                    value={selectedChainId}
                    onChange={onSelectChainId}
                    label={t('Фильтр цепочки')}
                    options={[
                      { value: '', label: t('Все цепочки') },
                      ...chainIds.map((id) => ({ value: id, label: id }))
                    ]}
                  />
                </div>
              </div>

              <form className="ca-board-form" onSubmit={handlePostBoard}>
                <div className="ca-board-form-row">
                  <label className="ca-field">
                    <span>{t('ID цепочки')}</span>
                    <input
                      type="text"
                      value={boardChain}
                      onChange={(event) => setBoardChain(event.target.value)}
                      placeholder={t('ID цепочки (авто)')}
                    />
                  </label>
                  <label className="ca-field">
                    <span>{t('Получатель')}</span>
                    <Select
                      value={boardTo}
                      onChange={setBoardTo}
                      label={t('Получатель')}
                      options={[
                        { value: '', label: t('Всем в цепочке') },
                        ...agents.map((item) => ({ value: item.id, label: item.name }))
                      ]}
                    />
                  </label>
                  <label className="ca-field">
                    <span>{t('Тема')}</span>
                    <input
                      type="text"
                      value={boardTopic}
                      onChange={(event) => setBoardTopic(event.target.value)}
                      placeholder={t('Тема')}
                    />
                  </label>
                </div>
                <label className="ca-field">
                  <span>{t('Сообщение')}</span>
                  <textarea
                    rows={2}
                    value={boardPayload}
                    onChange={(event) => setBoardPayload(event.target.value)}
                    placeholder={t('Сообщение на доску контекста…')}
                  />
                </label>
                <button type="submit" className="primary" disabled={postingBoard || !boardPayload.trim()}>
                  <MessageSquarePlus size={14} aria-hidden="true" />
                  <span>{t('Опубликовать')}</span>
                </button>
              </form>

              <div className="ca-board-list">
                {visibleBoard.length === 0 ? (
                  <div className="ca-empty-compact">{t('На доске пока нет записей')}</div>
                ) : (
                  visibleBoard.map((entry) => {
                    const targetAgent = agents.find((a) => a.id === entry.to_agent_id)
                    return (
                      <article key={entry.id} className="ca-board-card">
                        <div className="ca-board-card-head">
                          <strong>
                            {entry.from_agent_name || entry.from_agent_id || t('Система')}
                            {' → '}
                            {targetAgent?.name || entry.to_agent_id || t('Все')}
                          </strong>
                          <span className="ca-chip">{entry.topic || 'note'}</span>
                          <span className="ca-chip ca-chip-muted">{entry.status}</span>
                          <span className="ca-time">{formatTimestamp(entry.created_at, locale)}</span>
                        </div>
                        <p className="ca-board-payload">{entry.payload}</p>
                        <div className="ca-board-card-foot">
                          <code>{entry.chain_id}</code>
                        </div>
                      </article>
                    )
                  })
                )}
              </div>
            </div>
          </div>
        )}

        {tab === 'history' && (
          <div className="ca-history-pane">
            <div className="ca-history-toolbar">
              <div className="ca-segmented" role="group" aria-label={t('Фильтр истории')}>
                <button
                  type="button"
                  className={historyScope === 'agent' ? 'active' : ''}
                  aria-pressed={historyScope === 'agent'}
                  onClick={() => setHistoryScope('agent')}
                >
                  {t('Этот агент')}
                </button>
                <button
                  type="button"
                  className={historyScope === 'all' ? 'active' : ''}
                  aria-pressed={historyScope === 'all'}
                  onClick={() => setHistoryScope('all')}
                >
                  {t('Все агенты')}
                </button>
              </div>
              <span className="ca-muted-note">
                {t('Показано: {count}', { count: visibleHistory.length })}
              </span>
            </div>

            {visibleHistory.length === 0 ? (
              <div className="ca-empty-card">
                <p>{t('История запусков пуста')}</p>
              </div>
            ) : (
              <div className="ca-runs-list">
                {visibleHistory.map((run) => (
                  <RunTraceCard
                    key={run.id}
                    run={run}
                    showAgentName={historyScope === 'all'}
                    locale={locale}
                    t={t}
                    onCancel={() => void onCancelRun(run.agent_id, run.id)}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  )
}

function RunTraceCard({
  run,
  showAgentName = false,
  locale,
  t,
  onCancel
}: {
  run: CustomAgentRun
  showAgentName?: boolean
  locale: string
  t: (source: string, values?: Record<string, string | number>) => string
  onCancel: () => void
}) {
  const status = runStatusMeta(run.status)
  const active = isActiveRunStatus(run.status)
  const duration = formatDuration(run.started_at, run.finished_at)

  return (
    <article className="ca-run-card">
      <header className="ca-run-head">
        <div className="ca-run-head-left">
          <span className={`ca-status-badge tone-${status.tone}`}>
            <span className="ca-status-dot" aria-hidden="true" />
            {t(status.label)}
          </span>
          {showAgentName && <strong>{run.agent_name || run.agent_id}</strong>}
          <span className="ca-chip ca-chip-muted">
            {t('Попытка #{count}', { count: run.attempt || 1 })}
          </span>
          {run.trigger_reason && <span className="ca-chip ca-chip-muted">{run.trigger_reason}</span>}
        </div>
        <div className="ca-run-head-right">
          {duration && <span className="ca-time">{duration}</span>}
          <span className="ca-time">{formatTimestamp(run.started_at, locale)}</span>
          {active && (
            <button type="button" className="danger ca-mini-btn" onClick={onCancel}>
              <Square size={12} aria-hidden="true" />
              <span>{t('Отменить')}</span>
            </button>
          )}
        </div>
      </header>

      {(run.resolved_model || run.auto_model_reason) && (
        <div className="ca-model-trace">
          {run.resolved_model && (
            <span className="ca-chip">
              {run.resolved_provider ? `${run.resolved_provider}/${run.resolved_model}` : run.resolved_model}
            </span>
          )}
          {run.resolved_reasoning && (
            <span className="ca-chip ca-chip-muted">
              {t('Рассуждения: {level}', { level: run.resolved_reasoning })}
            </span>
          )}
          {run.auto_model_reason && <span className="ca-model-reason">{run.auto_model_reason}</span>}
        </div>
      )}

      {run.input_prompt && (
        <div className="ca-block">
          <span className="ca-block-label">{t('Задача')}</span>
          <p className="ca-prompt-preview">{run.input_prompt}</p>
        </div>
      )}

      {run.monitor_diff && (
        <div className="ca-block">
          <span className="ca-block-label">{t('Дифф мониторинга')}</span>
          <pre className="ca-code-box">{run.monitor_diff}</pre>
        </div>
      )}

      {run.steps && run.steps.length > 0 && (
        <div className="ca-block">
          <span className="ca-block-label">{t('Шаги выполнения')}</span>
          <ul className="ca-steps-list">
            {run.steps.map((step, index) => (
              <li key={`${step.timestamp}-${index}`} className={`ca-step-item status-${step.status}`}>
                <span className="ca-step-kind">{step.kind}</span>
                <div className="ca-step-body">
                  <strong>{step.title}</strong>
                  {step.detail && <span>{step.detail}</span>}
                </div>
                <span className="ca-time">{formatTimestamp(step.timestamp, locale)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {run.deterministic_report && (
        <div className="ca-block">
          <span className="ca-block-label">{t('Детерминированные проверки')}</span>
          <pre className="ca-code-box">{run.deterministic_report}</pre>
        </div>
      )}

      {run.verifier_verdict && (
        <div className={`ca-verdict-banner ${run.verifier_verdict === 'PASS' ? 'pass' : 'fail'}`}>
          <div className="ca-verdict-head">
            {run.verifier_verdict === 'PASS' ? (
              <CheckCircle2 size={15} aria-hidden="true" />
            ) : (
              <TriangleAlert size={15} aria-hidden="true" />
            )}
            <strong>{`VERDICT: ${run.verifier_verdict}`}</strong>
          </div>
          {run.verifier_feedback && <pre className="ca-verdict-text">{run.verifier_feedback}</pre>}
        </div>
      )}

      {run.output_text && (
        <div className="ca-block">
          <span className="ca-block-label">{t('Результат')}</span>
          <pre className="ca-output-box">{run.output_text}</pre>
        </div>
      )}

      {run.error && (
        <div className="ca-error-inline" role="alert">
          {run.error}
        </div>
      )}
    </article>
  )
}
