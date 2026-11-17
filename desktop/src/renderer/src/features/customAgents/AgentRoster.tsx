import { Plus, Search, X } from 'lucide-react'
import type { CustomAgent, ModelOption } from '../../../../shared/customAgents'
import { useI18n } from '../../i18n'
import {
  agentRoleLabel,
  agentStatusMeta,
  filterCustomAgents,
  modelSummary,
  type RoleFilter
} from './presentation'

export function AgentRoster({
  agents,
  models,
  selectedId,
  query,
  roleFilter,
  loading,
  onSelect,
  onQueryChange,
  onRoleFilterChange,
  onToggleEnabled,
  onOpenCreate
}: {
  agents: CustomAgent[]
  models: ModelOption[]
  selectedId: string
  query: string
  roleFilter: RoleFilter
  loading: boolean
  onSelect: (id: string) => void
  onQueryChange: (query: string) => void
  onRoleFilterChange: (role: RoleFilter) => void
  onToggleEnabled: (agent: CustomAgent) => void
  onOpenCreate: () => void
}) {
  const { t } = useI18n()
  const filtered = filterCustomAgents(agents, query, roleFilter)
  const roles: { id: RoleFilter; label: string }[] = [
    { id: 'all', label: t('Все') },
    { id: 'orchestrator', label: t('Координатор') },
    { id: 'worker', label: t('Воркеры') },
    { id: 'verifier', label: t('Верификаторы') }
  ]

  return (
    <aside className="ca-roster" aria-label={t('Список агентов')}>
      <header className="ca-pane-head">
        <div className="ca-pane-title-row">
          <h2>{t('Свои агенты')}</h2>
          <span className="ca-count">{agents.length}</span>
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
        ) : filtered.length === 0 ? (
          <div className="ca-empty-compact">
            <p>{agents.length === 0 ? t('Агентов пока нет') : t('Ничего не найдено')}</p>
            {agents.length === 0 && (
              <button type="button" onClick={onOpenCreate}>
                <Plus size={14} />
                {t('Создать агента')}
              </button>
            )}
          </div>
        ) : (
          filtered.map((agent) => {
            const status = agentStatusMeta(agent.status)
            const model = modelSummary(agent, models)
            const selected = agent.id === selectedId
            return (
              <div
                key={agent.id}
                className={`ca-agent-item ${selected ? 'selected' : ''} ${agent.enabled ? '' : 'disabled'}`}
              >
                <button
                  type="button"
                  role="option"
                  aria-selected={selected}
                  className="ca-agent-select"
                  onClick={() => onSelect(agent.id)}
                >
                  <strong className="ca-agent-name" title={agent.name}>
                    {agent.name}
                  </strong>
                  <div className="ca-agent-meta-row">
                    <span className={`ca-status-badge tone-${status.tone}`}>
                      <span className="ca-status-dot" aria-hidden="true" />
                      {t(status.label)}
                    </span>
                    <span className="ca-chip">{t(agentRoleLabel(agent.role))}</span>
                    <span className="ca-chip ca-chip-muted" title={model}>
                      {model === 'Авто' ? t('Авто') : model}
                    </span>
                    <span className="ca-slug">/{agent.slug}</span>
                  </div>
                </button>
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
            )
          })
        )}
      </div>
    </aside>
  )
}
