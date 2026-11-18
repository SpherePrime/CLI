import { useEffect, useMemo, useRef, useState } from 'react'
import { Bot, Link2, Plus, Search, ShieldCheck, Sparkles, X } from 'lucide-react'
import type {
  CustomAgent,
  CustomWorkflow,
  ModelOption
} from '../../../../shared/customAgents'
import { useI18n } from '../../i18n'
import {
  agentRoleLabel,
  agentStatusMeta,
  filterCustomAgents,
  modelSummary,
  type RoleFilter
} from './presentation'

function RoleGlyph({ role }: { role: string }) {
  if (role === 'orchestrator') return <Sparkles size={13} aria-hidden="true" />
  if (role === 'verifier') return <ShieldCheck size={13} aria-hidden="true" />
  return <Bot size={13} aria-hidden="true" />
}

export function AgentRoster({
  agents,
  workflows,
  activeWorkflowId,
  models,
  selectedId,
  query,
  roleFilter,
  loading,
  onSelect,
  onQueryChange,
  onRoleFilterChange,
  onToggleEnabled,
  onAttachToWorkflow,
  onOpenCreate
}: {
  agents: CustomAgent[]
  workflows: CustomWorkflow[]
  activeWorkflowId: string
  models: ModelOption[]
  selectedId: string
  query: string
  roleFilter: RoleFilter
  loading: boolean
  onSelect: (id: string) => void
  onQueryChange: (query: string) => void
  onRoleFilterChange: (role: RoleFilter) => void
  onToggleEnabled: (agent: CustomAgent) => void
  onAttachToWorkflow: (agentId: string) => void
  onOpenCreate: () => void
}) {
  const { t } = useI18n()
  const autoScopedRef = useRef(workflows.length > 1)
  const [scope, setScope] = useState<'workflow' | 'all'>(() =>
    workflows.length > 1 ? 'workflow' : 'all'
  )

  useEffect(() => {
    if (workflows.length > 1 && !autoScopedRef.current) {
      autoScopedRef.current = true
      setScope('workflow')
    }
  }, [workflows.length])

  const workflowNames = useMemo(
    () => new Map(workflows.map((wf) => [wf.id, wf.name])),
    [workflows]
  )

  const filtered = filterCustomAgents(agents, query, roleFilter)
  const inWorkflow = filtered.filter(
    (agent) => (agent.workflow_id || 'default') === activeWorkflowId
  )
  const outsideWorkflow = filtered.filter(
    (agent) => (agent.workflow_id || 'default') !== activeWorkflowId
  )
  const visibleList = scope === 'workflow' ? inWorkflow : [...inWorkflow, ...outsideWorkflow]

  const roles: { id: RoleFilter; label: string }[] = [
    { id: 'all', label: t('Все') },
    { id: 'orchestrator', label: t('Координатор') },
    { id: 'worker', label: t('Воркеры') },
    { id: 'verifier', label: t('Верификаторы') }
  ]

  const renderItem = (agent: CustomAgent, isExternal: boolean) => {
    const status = agentStatusMeta(agent.status)
    const model = modelSummary(agent, models)
    const selected = agent.id === selectedId
    const sourceWorkflowName = workflowNames.get(agent.workflow_id || 'default')

    return (
      <div
        key={agent.id}
        data-role={agent.role}
        className={`ca-agent-item role-${agent.role} ${selected ? 'selected' : ''} ${agent.enabled ? '' : 'disabled'}`}
      >
        <button
          type="button"
          role="option"
          aria-selected={selected}
          className="ca-agent-select"
          onClick={() => onSelect(agent.id)}
        >
          <div className="ca-agent-name-row">
            <span
              className={`ca-roster-role-icon role-${agent.role}`}
              title={t(agentRoleLabel(agent.role))}
            >
              <RoleGlyph role={agent.role} />
            </span>
            <strong className="ca-agent-name" title={agent.name}>
              {agent.name}
            </strong>
          </div>
          <div className="ca-agent-meta-row">
            <span className={`ca-status-badge tone-${status.tone}`}>
              <span className="ca-status-dot" aria-hidden="true" />
              {t(status.label)}
            </span>
            <span className="ca-chip ca-chip-muted" title={model}>
              {model === 'Авто' ? t('Авто') : model}
            </span>
            {isExternal && sourceWorkflowName && (
              <span className="ca-chip ca-chip-muted" title={sourceWorkflowName}>
                {sourceWorkflowName}
              </span>
            )}
          </div>
        </button>
        <div className="ca-agent-item-controls">
          {isExternal && (
            <button
              type="button"
              className="ca-roster-attach-btn"
              title={t('Перенести в текущую цепь')}
              onClick={() => onAttachToWorkflow(agent.id)}
            >
              <Link2 size={11} aria-hidden="true" />
              <span>{t('В эту цепь')}</span>
            </button>
          )}
          <button
            type="button"
            role="switch"
            aria-checked={agent.enabled}
            aria-label={
              agent.enabled
                ? t('Отключить агента {name}', { name: agent.name })
                : t('Включить агента {name}', { name: agent.name })
            }
            title={agent.enabled ? t('Выключить') : t('Включить')}
            className={`ca-switch ${agent.enabled ? 'on' : ''}`}
            onClick={() => onToggleEnabled(agent)}
          >
            <span className="ca-switch-thumb" />
          </button>
        </div>
      </div>
    )
  }

  return (
    <aside className="ca-roster" aria-label={t('Список агентов')}>
      <header className="ca-pane-head">
        <div className="ca-pane-title-row">
          <h2>{t('Узлы и агенты')}</h2>
          <span className="ca-count">{visibleList.length}</span>
        </div>
        <button
          type="button"
          className="primary ca-new-btn"
          onClick={onOpenCreate}
        >
          <Plus size={15} aria-hidden="true" />
          <span>{t('Новый агент')}</span>
        </button>
      </header>

      <div className="ca-roster-controls">
        <div className="ca-segmented ca-roster-scope" role="group" aria-label={t('Фильтр цепочки')}>
          <button
            type="button"
            className={scope === 'workflow' ? 'active' : ''}
            aria-pressed={scope === 'workflow'}
            onClick={() => setScope('workflow')}
          >
            {t('Эта цепь')}
          </button>
          <button
            type="button"
            className={scope === 'all' ? 'active' : ''}
            aria-pressed={scope === 'all'}
            onClick={() => setScope('all')}
          >
            {t('Все агенты')}
          </button>
        </div>

        <label className="ca-search-box">
          <Search size={14} aria-hidden="true" />
          <input
            type="search"
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={t('Поиск агентов…')}
            aria-label={t('Поиск агентов')}
          />
          {query && (
            <button
              type="button"
              className="icon small"
              aria-label={t('Очистить поиск')}
              onClick={() => onQueryChange('')}
            >
              <X size={12} />
            </button>
          )}
        </label>

        <div className="ca-filter-pills" role="group" aria-label={t('Фильтр по роли')}>
          {roles.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`ca-pill-btn ${roleFilter === item.id ? 'active' : ''}`}
              aria-pressed={roleFilter === item.id}
              onClick={() => onRoleFilterChange(item.id)}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div className="ca-roster-list" role="listbox" aria-label={t('Агенты')}>
        {loading && agents.length === 0 ? (
          <div className="ca-empty-compact">{t('Загружаем агентов…')}</div>
        ) : visibleList.length === 0 ? (
          <div className="ca-empty-compact">
            <p>
              {scope === 'workflow' && agents.length > 0
                ? t('В этой цепочке пока нет узлов')
                : agents.length === 0
                  ? t('Агентов пока нет')
                  : t('Ничего не найдено')}
            </p>
            <div className="ca-empty-actions">
              {scope === 'workflow' && outsideWorkflow.length > 0 && (
                <button type="button" onClick={() => setScope('all')}>
                  {t('Показать всех')}
                </button>
              )}
              <button type="button" onClick={onOpenCreate}>
                <Plus size={14} />
                {t('Создать агента')}
              </button>
            </div>
          </div>
        ) : scope === 'workflow' ? (
          inWorkflow.map((agent) => renderItem(agent, false))
        ) : (
          <>
            {inWorkflow.map((agent) => renderItem(agent, false))}
            {outsideWorkflow.length > 0 && (
              <div className="ca-roster-group-label">{t('Другие цепочки')}</div>
            )}
            {outsideWorkflow.map((agent) => renderItem(agent, true))}
          </>
        )}
      </div>
    </aside>
  )
}
