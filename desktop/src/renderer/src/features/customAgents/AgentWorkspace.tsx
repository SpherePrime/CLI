import { useEffect, useMemo, useState } from 'react'
import {
  CheckCircle2,
  Layers,
  MessageSquarePlus,
  Play,
  Radar,
  Send,
  Settings2,
  Square,
  TriangleAlert,
  X
} from 'lucide-react'
import type {
  BlackboardEntry,
  CustomAgent,
  CustomAgentRun,
  ModelOption,
  ProbeResult,
  UpdateCustomAgentInput
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
import { WorkflowCanvas, type PendingWorkflowLink } from './WorkflowCanvas'

export type WorkspaceTab = 'chain' | 'tasks' | 'runs'
export type RunsSubTab = 'history' | 'board'

export function AgentWorkspace({
  agent,
  agents,
  models,
  runs,
  allRuns,
  blackboard,
  selectedChainId,
  tab,
  requestedRunsSubTab,
  onTabChange,
  onSelectChainId,
  onSelectAgent,
  onConfigureAgent,
  onRun,
  onCancelRun,
  onProbe,
  onPostBlackboard,
  onUpdateAgent,
  onOpenCreate,
  onOpenSwarm
}: {
  agent: CustomAgent
  agents: CustomAgent[]
  models: ModelOption[]
  runs: CustomAgentRun[]
  allRuns: CustomAgentRun[]
  blackboard: BlackboardEntry[]
  selectedChainId: string
  tab: WorkspaceTab
  requestedRunsSubTab?: RunsSubTab
  onTabChange: (tab: WorkspaceTab) => void
  onSelectChainId: (chainId: string) => void
  onSelectAgent: (agentId: string) => void
  onConfigureAgent: (agentId: string) => void
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
  onUpdateAgent: (agentId: string, patch: UpdateCustomAgentInput) => Promise<CustomAgent>
  onOpenCreate: (pendingLink?: PendingWorkflowLink) => void
  onOpenSwarm: () => void
}) {
  const { t, locale } = useI18n()
  const [runsSubTab, setRunsSubTab] = useState<RunsSubTab>('history')
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
  const [actionError, setActionError] = useState('')

  useEffect(() => {
    setProbeResult(null)
    setActionError('')
  }, [agent.id])

  useEffect(() => {
    if (selectedChainId) setBoardChain(selectedChainId)
  }, [selectedChainId])

  useEffect(() => {
    if (requestedRunsSubTab) setRunsSubTab(requestedRunsSubTab)
  }, [requestedRunsSubTab])

  const statusMeta = agentStatusMeta(agent.status)
  const activeRun = useMemo(
    () => runs.find((run) => isActiveRunStatus(run.status)) || null,
    [runs]
  )
  const latestRun = runs[0] || null

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

  const handleDispatch = async (
    targetAgentId: string,
    taskPrompt: string,
    triggerReason = 'manual'
  ) => {
    if (dispatching) return
    setDispatching(true)
    setActionError('')
    try {
      await onRun({
        agentId: targetAgentId,
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

  if (tab === 'chain') {
    return (
      <section className="ca-workspace is-canvas" aria-label={t('Конструктор цепей')}>
        {actionError && (
          <div className="ca-error-banner ca-canvas-error" role="alert">
            <TriangleAlert size={15} aria-hidden="true" />
            <span>{actionError}</span>
          </div>
        )}
        <WorkflowCanvas
          agents={agents}
          models={models}
          selectedAgentId={agent.id}
          onSelectAgent={onSelectAgent}
          onConfigureAgent={onConfigureAgent}
          onRunAgent={(targetId) => {
            void handleDispatch(targetId, '')
            onTabChange('tasks')
          }}
          onUpdateAgent={onUpdateAgent}
          onOpenCreate={onOpenCreate}
          onOpenSwarm={onOpenSwarm}
        />
      </section>
    )
  }

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
          <button type="button" onClick={() => onConfigureAgent(agent.id)}>
            <Settings2 size={14} aria-hidden="true" />
            <span>{t('Настроить')}</span>
          </button>
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
              onClick={() => void handleDispatch(agent.id, prompt || '')}
            >
              <Play size={14} aria-hidden="true" />
              <span>{dispatching ? t('Запуск…') : t('Запустить')}</span>
            </button>
          )}
        </div>
      </header>

      {actionError && (
        <div className="ca-error-banner" role="alert">
          <TriangleAlert size={15} aria-hidden="true" />
          <span>{actionError}</span>
        </div>
      )}

      {probeResult && (
        <div
          className={`ca-probe-banner ca-probe-pinned ${probeResult.error ? 'tone-red' : probeResult.changed ? 'tone-amber' : 'tone-green'}`}
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

      <div className="ca-workspace-body">
        <div className="ca-workspace-col">
          {tab === 'tasks' && (
            <div className="ca-tasks-pane">
              <form
                className="ca-prompt-box"
                onSubmit={(event) => {
                  event.preventDefault()
                  void handleDispatch(agent.id, prompt)
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

          {tab === 'runs' && (
            <div className="ca-history-pane">
              <div className="ca-runs-topbar">
                <div className="ca-segmented ca-runs-switch" role="tablist" aria-label={t('Режим просмотров')}>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={runsSubTab === 'history'}
                    className={runsSubTab === 'history' ? 'active' : ''}
                    onClick={() => setRunsSubTab('history')}
                  >
                    {t('История запусков')}
                  </button>
                  <button
                    type="button"
                    role="tab"
                    aria-selected={runsSubTab === 'board'}
                    className={runsSubTab === 'board' ? 'active' : ''}
                    onClick={() => setRunsSubTab('board')}
                  >
                    {t('Общая доска (Blackboard)')}
                  </button>
                </div>

                <button type="button" className="ca-mini-btn" onClick={onOpenSwarm}>
                  <Layers size={13} aria-hidden="true" />
                  <span>{t('Запустить рой (Swarm)')}</span>
                </button>
              </div>

              {runsSubTab === 'history' ? (
                <>
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
                </>
              ) : (
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
                    <button
                      type="submit"
                      className="primary"
                      disabled={postingBoard || !boardPayload.trim()}
                    >
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
                              <span className="ca-time">
                                {formatTimestamp(entry.created_at, locale)}
                              </span>
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
              )}
            </div>
          )}
        </div>
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
