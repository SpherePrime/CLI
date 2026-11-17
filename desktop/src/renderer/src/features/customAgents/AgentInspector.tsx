import { useEffect, useMemo, useState } from 'react'
import { Plus, Radar, Save, Trash2 } from 'lucide-react'
import type {
  CustomAgent,
  CustomAgentModelMode,
  CustomAgentRole,
  CustomAgentRun,
  CustomAgentTriggerType,
  DeterministicCheck,
  DeterministicCheckKind,
  ModelOption,
  ProbeResult
} from '../../../../shared/customAgents'
import type { Project } from '../../../../shared/contracts'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import {
  buildAgentDraft,
  isAgentDraftDirty,
  type AgentDraft
} from './presentation'

export function AgentInspector({
  agent,
  agents,
  models,
  projects,
  latestRun,
  busy,
  onSave,
  onDelete,
  onProbe
}: {
  agent: CustomAgent
  agents: CustomAgent[]
  models: ModelOption[]
  projects: Project[]
  latestRun: CustomAgentRun | null
  busy: boolean
  onSave: (agentId: string, patch: Partial<CustomAgent>) => Promise<CustomAgent>
  onDelete: (agentId: string) => Promise<unknown>
  onProbe: (agentId: string) => Promise<ProbeResult>
}) {
  const { t } = useI18n()
  const [draft, setDraft] = useState<AgentDraft>(() => buildAgentDraft(agent))
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [probing, setProbing] = useState(false)
  const [probeState, setProbeState] = useState<ProbeResult | null>(null)
  const [saveError, setSaveError] = useState('')

  useEffect(() => {
    setDraft(buildAgentDraft(agent))
    setConfirmDelete(false)
    setProbeState(null)
    setSaveError('')
  }, [agent])

  const dirty = useMemo(() => isAgentDraftDirty(agent, draft), [agent, draft])

  const providers = useMemo(() => {
    const map = new Map<string, string>()
    for (const model of models) {
      if (model.provider_id && !map.has(model.provider_id)) {
        map.set(model.provider_id, model.provider_name || model.provider_id)
      }
    }
    return [...map.entries()].map(([id, name]) => ({ value: id, label: name }))
  }, [models])

  const providerModels = useMemo(() => {
    const filtered = draft.provider_id
      ? models.filter((m) => m.provider_id === draft.provider_id)
      : models
    return filtered.map((m) => ({
      value: m.model_id,
      label: m.locked ? `${m.name || m.model_id} (${m.plan_label || 'Locked'})` : m.name || m.model_id
    }))
  }, [models, draft.provider_id])

  const workspaceOptions = useMemo(() => {
    const seen = new Set<string>([''])
    const list = [{ value: '', label: t('Глобальный') }]
    for (const proj of projects) {
      if (proj.kind === 'project' && proj.path && !seen.has(proj.path)) {
        seen.add(proj.path)
        list.push({ value: proj.path, label: proj.path.split(/[\\/]/).pop() || proj.path })
      }
    }
    if (draft.workspace_path && !seen.has(draft.workspace_path)) {
      list.push({ value: draft.workspace_path, label: draft.workspace_path })
    }
    return list
  }, [projects, draft.workspace_path, t])

  const otherAgents = useMemo(
    () => agents.filter((item) => item.id !== agent.id),
    [agents, agent.id]
  )

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault()
    if (busy) return
    setSaveError('')
    try {
      await onSave(agent.id, {
        name: draft.name.trim() || agent.name,
        slug: draft.slug.trim() || agent.slug,
        role: draft.role,
        description: draft.description,
        workspace_path: draft.workspace_path.trim(),
        model_mode: draft.model_mode,
        provider_id: draft.provider_id,
        model_id: draft.model_id,
        reasoning_mode: draft.reasoning_mode,
        auto_model_hint: draft.auto_model_hint,
        system_prompt: draft.system_prompt,
        memory: draft.memory,
        trigger_type: draft.trigger_type,
        interval_minutes: Number(draft.interval_minutes) || 0,
        cron_expr: draft.cron_expr.trim(),
        monitor_kind: draft.monitor_kind,
        monitor_target: draft.monitor_target.trim(),
        acceptance_criteria: draft.acceptance_criteria,
        deterministic_checks: draft.deterministic_checks,
        verifier_agent_id: draft.verifier_agent_id,
        next_agent_id: draft.next_agent_id,
        max_rework_attempts: Math.max(1, Number(draft.max_rework_attempts) || 3),
        strict_verify: draft.strict_verify
      })
    } catch (cause) {
      setSaveError(String((cause as Error)?.message || cause))
    }
  }

  const handleProbeNow = async () => {
    if (probing) return
    setProbing(true)
    setSaveError('')
    try {
      if (dirty) {
        await onSave(agent.id, {
          monitor_kind: draft.monitor_kind,
          monitor_target: draft.monitor_target.trim()
        })
      }
      const res = await onProbe(agent.id)
      setProbeState(res)
    } catch (cause) {
      setSaveError(String((cause as Error)?.message || cause))
    } finally {
      setProbing(false)
    }
  }

  const addCheck = () => {
    const next: DeterministicCheck = {
      id: `chk-${draft.deterministic_checks.length + 1}`,
      kind: 'regex',
      target: 'output',
      pattern: '',
      expected: '',
      required: true
    }
    setDraft((prev) => ({
      ...prev,
      deterministic_checks: [...prev.deterministic_checks, next]
    }))
  }

  const updateCheck = (index: number, patch: Partial<DeterministicCheck>) => {
    setDraft((prev) => ({
      ...prev,
      deterministic_checks: prev.deterministic_checks.map((item, idx) =>
        idx === index ? { ...item, ...patch } : item
      )
    }))
  }

  const removeCheck = (index: number) => {
    setDraft((prev) => ({
      ...prev,
      deterministic_checks: prev.deterministic_checks.filter((_, idx) => idx !== index)
    }))
  }

  return (
    <aside className="ca-inspector" aria-label={t('Настройки агента')}>
      <form className="ca-inspector-form" onSubmit={handleSave}>
        <header className="ca-pane-head">
          <div className="ca-pane-title-row">
            <h2>{t('Параметры')}</h2>
            {dirty && <span className="ca-dirty-pill">{t('Изменено')}</span>}
          </div>
          <div className="ca-inspector-head-actions">
            <button type="submit" className="primary" disabled={busy || !dirty}>
              <Save size={14} aria-hidden="true" />
              <span>{t('Сохранить')}</span>
            </button>
          </div>
        </header>

        {saveError && (
          <div className="ca-error-banner" role="alert">
            <span>{saveError}</span>
          </div>
        )}

        <div className="ca-inspector-scroll">
          <section className="ca-section">
            <h3>{t('Основное')}</h3>
            <label className="ca-field">
              <span>{t('Название')}</span>
              <input
                type="text"
                value={draft.name}
                onChange={(event) => setDraft({ ...draft, name: event.target.value })}
              />
            </label>
            <div className="ca-form-row">
              <label className="ca-field">
                <span>{t('Идентификатор (slug)')}</span>
                <input
                  type="text"
                  value={draft.slug}
                  onChange={(event) => setDraft({ ...draft, slug: event.target.value })}
                />
              </label>
              <label className="ca-field">
                <span>{t('Роль')}</span>
                <Select
                  value={draft.role}
                  onChange={(value) => setDraft({ ...draft, role: value as CustomAgentRole })}
                  label={t('Роль')}
                  options={[
                    { value: 'worker', label: t('Воркер') },
                    { value: 'orchestrator', label: t('Координатор') },
                    { value: 'verifier', label: t('Верификатор') }
                  ]}
                />
              </label>
            </div>
            <label className="ca-field">
              <span>{t('Описание')}</span>
              <textarea
                rows={3}
                value={draft.description}
                onChange={(event) => setDraft({ ...draft, description: event.target.value })}
              />
            </label>
            <label className="ca-field">
              <span>{t('Привязка к проекту')}</span>
              <Select
                value={draft.workspace_path}
                onChange={(value) => setDraft({ ...draft, workspace_path: value })}
                label={t('Привязка к проекту')}
                options={workspaceOptions}
              />
            </label>
            <label className="ca-field">
              <span>{t('Путь рабочей папки')}</span>
              <input
                type="text"
                value={draft.workspace_path}
                onChange={(event) => setDraft({ ...draft, workspace_path: event.target.value })}
                placeholder={t('Оставьте пустым для глобального режима')}
              />
            </label>
          </section>

          <section className="ca-section">
            <h3>{t('Модель и рассуждения')}</h3>
            <div className="ca-segmented" role="group" aria-label={t('Режим выбора модели')}>
              <button
                type="button"
                className={draft.model_mode === 'auto' ? 'active' : ''}
                aria-pressed={draft.model_mode === 'auto'}
                onClick={() => setDraft({ ...draft, model_mode: 'auto' as CustomAgentModelMode })}
              >
                {t('Авто-подбор ИИ')}
              </button>
              <button
                type="button"
                className={draft.model_mode === 'manual' ? 'active' : ''}
                aria-pressed={draft.model_mode === 'manual'}
                onClick={() => {
                  const firstModel = models[0]
                  setDraft({
                    ...draft,
                    model_mode: 'manual',
                    provider_id: draft.provider_id || firstModel?.provider_id || '',
                    model_id: draft.model_id || firstModel?.model_id || '',
                    reasoning_mode:
                      draft.reasoning_mode || firstModel?.default_reasoning_mode || 'medium'
                  })
                }}
              >
                {t('Вручную')}
              </button>
            </div>

            {draft.model_mode === 'auto' ? (
              <>
                <label className="ca-field">
                  <span>{t('Ориентир подбора')}</span>
                  <Select
                    value={draft.auto_model_hint}
                    onChange={(value) => setDraft({ ...draft, auto_model_hint: value })}
                    label={t('Ориентир подбора')}
                    options={[
                      { value: '', label: t('По сложности задачи') },
                      { value: 'fast', label: t('Быстрая (экономичная)') },
                      { value: 'balanced', label: t('Сбалансированная') },
                      { value: 'reasoning', label: t('Глубокие рассуждения') },
                      { value: 'flagship', label: t('Флагманская') }
                    ]}
                  />
                </label>
                {latestRun?.resolved_model && (
                  <div className="ca-info-banner">
                    <strong>
                      {t('Последний выбор: {model}', {
                        model: latestRun.resolved_provider
                          ? `${latestRun.resolved_provider}/${latestRun.resolved_model}`
                          : latestRun.resolved_model
                      })}
                    </strong>
                    {latestRun.resolved_reasoning && (
                      <span>
                        {t('Уровень рассуждений: {level}', {
                          level: latestRun.resolved_reasoning
                        })}
                      </span>
                    )}
                    {latestRun.auto_model_reason && <span>{latestRun.auto_model_reason}</span>}
                  </div>
                )}
              </>
            ) : (
              <>
                <label className="ca-field">
                  <span>{t('Провайдер')}</span>
                  <Select
                    value={draft.provider_id}
                    onChange={(value) => {
                      const nextModel = models.find((m) => m.provider_id === value)
                      setDraft({
                        ...draft,
                        provider_id: value,
                        model_id: nextModel?.model_id || ''
                      })
                    }}
                    label={t('Провайдер')}
                    options={
                      providers.length
                        ? providers
                        : [{ value: draft.provider_id || '', label: draft.provider_id || t('Не выбран') }]
                    }
                  />
                </label>
                <label className="ca-field">
                  <span>{t('Модель')}</span>
                  <Select
                    value={draft.model_id}
                    onChange={(value) => setDraft({ ...draft, model_id: value })}
                    label={t('Модель')}
                    options={
                      providerModels.length
                        ? providerModels
                        : [{ value: draft.model_id || '', label: draft.model_id || t('Не выбрана') }]
                    }
                  />
                </label>
                <label className="ca-field">
                  <span>{t('Уровень рассуждений')}</span>
                  <Select
                    value={draft.reasoning_mode || 'default'}
                    onChange={(value) =>
                      setDraft({ ...draft, reasoning_mode: value === 'default' ? '' : value })
                    }
                    label={t('Уровень рассуждений')}
                    options={[
                      { value: 'default', label: t('По умолчанию') },
                      { value: 'off', label: t('Выкл.') },
                      { value: 'low', label: t('Низкий') },
                      { value: 'medium', label: t('Средний') },
                      { value: 'high', label: t('Высокий') },
                      { value: 'xhigh', label: t('Очень высокий') },
                      { value: 'max', label: t('Максимум') }
                    ]}
                  />
                </label>
              </>
            )}
          </section>

          <section className="ca-section">
            <h3>{t('Промпт и память')}</h3>
            <label className="ca-field">
              <span>{t('Системный промпт')}</span>
              <textarea
                rows={4}
                value={draft.system_prompt}
                onChange={(event) => setDraft({ ...draft, system_prompt: event.target.value })}
              />
            </label>
            <label className="ca-field">
              <span>{t('Постоянная память (MEMORY.md)')}</span>
              <textarea
                rows={4}
                value={draft.memory}
                onChange={(event) => setDraft({ ...draft, memory: event.target.value })}
              />
            </label>
          </section>

          <section className="ca-section">
            <h3>{t('Триггер и Monitor Mode')}</h3>
            <label className="ca-field">
              <span>{t('Тип запуска')}</span>
              <Select
                value={draft.trigger_type}
                onChange={(value) =>
                  setDraft({ ...draft, trigger_type: value as CustomAgentTriggerType })
                }
                label={t('Тип запуска')}
                options={[
                  { value: 'manual', label: t('Вручную') },
                  { value: 'interval', label: t('По интервалу') },
                  { value: 'cron', label: t('Расписание Cron') },
                  { value: 'event', label: t('По событию') },
                  { value: 'monitor', label: t('Мониторинг изменений') }
                ]}
              />
            </label>

            {(draft.trigger_type === 'interval' || draft.trigger_type === 'monitor') && (
              <label className="ca-field">
                <span>{t('Интервал (минуты)')}</span>
                <input
                  type="number"
                  min={1}
                  max={10080}
                  value={draft.interval_minutes || 5}
                  onChange={(event) =>
                    setDraft({ ...draft, interval_minutes: Number(event.target.value) || 0 })
                  }
                />
              </label>
            )}

            {draft.trigger_type === 'cron' && (
              <label className="ca-field">
                <span>{t('Cron-выражение')}</span>
                <input
                  type="text"
                  value={draft.cron_expr}
                  onChange={(event) => setDraft({ ...draft, cron_expr: event.target.value })}
                  placeholder="*/15 * * * *"
                />
              </label>
            )}

            <label className="ca-field">
              <span>{t('Тип зонда мониторинга')}</span>
              <Select
                value={draft.monitor_kind}
                onChange={(value) =>
                  setDraft({ ...draft, monitor_kind: value as '' | 'command' | 'http' | 'file' })
                }
                label={t('Тип зонда мониторинга')}
                options={[
                  { value: '', label: t('Не используется') },
                  { value: 'http', label: 'HTTP URL' },
                  { value: 'file', label: t('Файл') },
                  { value: 'command', label: t('Команда') }
                ]}
              />
            </label>

            {draft.monitor_kind && (
              <>
                <label className="ca-field">
                  <span>{t('Цель зонда')}</span>
                  <input
                    type="text"
                    value={draft.monitor_target}
                    onChange={(event) => setDraft({ ...draft, monitor_target: event.target.value })}
                    placeholder={
                      draft.monitor_kind === 'http'
                        ? 'https://...'
                        : draft.monitor_kind === 'file'
                          ? 'path/to/file'
                          : 'git status --short'
                    }
                  />
                </label>

                <div className="ca-hash-row">
                  <span className="ca-muted-note">SHA-256:</span>
                  <code title={agent.monitor_last_hash || ''}>
                    {agent.monitor_last_hash ? agent.monitor_last_hash.slice(0, 16) : '—'}
                  </code>
                  <button
                    type="button"
                    disabled={probing || !draft.monitor_target.trim()}
                    onClick={() => void handleProbeNow()}
                  >
                    <Radar size={13} aria-hidden="true" />
                    <span>{probing ? t('Проверка…') : t('Проверить сейчас')}</span>
                  </button>
                </div>

                {probeState && (
                  <div className="ca-probe-inline">
                    <strong>
                      {probeState.error
                        ? probeState.error
                        : probeState.changed
                          ? t('Обнаружены изменения')
                          : t('Состояние не изменилось')}
                    </strong>
                    {probeState.diff && <pre className="ca-code-box">{probeState.diff}</pre>}
                  </div>
                )}
              </>
            )}
          </section>

          <section className="ca-section">
            <h3>{t('Жёсткий контроль (Верификатор)')}</h3>
            <div className="ca-toggle-row">
              <div className="ca-toggle-copy">
                <strong>{t('Строгая приёмка Verifier')}</strong>
                <span className="ca-muted-note">
                  {t('Проверять результат даже без списка критериев')}
                </span>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={draft.strict_verify}
                aria-label={t('Строгая приёмка Verifier')}
                className={`ca-switch ${draft.strict_verify ? 'on' : ''}`}
                onClick={() => setDraft({ ...draft, strict_verify: !draft.strict_verify })}
              >
                <span className="ca-switch-thumb" />
              </button>
            </div>
            <label className="ca-field">
              <span>{t('Критерии приёмки')}</span>
              <textarea
                rows={3}
                value={draft.acceptance_criteria}
                onChange={(event) =>
                  setDraft({ ...draft, acceptance_criteria: event.target.value })
                }
                placeholder={t('Что именно должен подтвердить верификатор')}
              />
            </label>

            <div className="ca-checks-head">
              <span>{t('Детерминированные проверки')}</span>
              <button type="button" onClick={addCheck}>
                <Plus size={13} aria-hidden="true" />
                <span>{t('Добавить')}</span>
              </button>
            </div>

            {draft.deterministic_checks.length > 0 && (
              <div className="ca-checks-list">
                {draft.deterministic_checks.map((check, index) => (
                  <div key={`${check.id}-${index}`} className="ca-check-card">
                    <div className="ca-form-row">
                      <label className="ca-field">
                        <span>{t('ID проверки')}</span>
                        <input
                          type="text"
                          value={check.id}
                          onChange={(event) => updateCheck(index, { id: event.target.value })}
                          placeholder="ID"
                        />
                      </label>
                      <label className="ca-field">
                        <span>{t('Тип')}</span>
                        <Select
                          value={check.kind}
                          onChange={(value) =>
                            updateCheck(index, { kind: value as DeterministicCheckKind })
                          }
                          label={t('Тип проверки')}
                          options={[
                            { value: 'regex', label: 'Regex' },
                            { value: 'json_fields', label: 'JSON поля' },
                            { value: 'file_exists', label: t('Файл существует') },
                            { value: 'command', label: t('Команда') }
                          ]}
                        />
                      </label>
                    </div>
                    <label className="ca-field">
                      <span>{t('Цель')}</span>
                      <input
                        type="text"
                        value={check.target}
                        onChange={(event) => updateCheck(index, { target: event.target.value })}
                        placeholder={t('Цель (output или путь)')}
                      />
                    </label>
                    <label className="ca-field">
                      <span>{t('Шаблон / поля')}</span>
                      <input
                        type="text"
                        value={check.pattern}
                        onChange={(event) => updateCheck(index, { pattern: event.target.value })}
                        placeholder={t('Шаблон / поля через запятую / команда')}
                      />
                    </label>
                    <div className="ca-check-foot">
                      <label className="ca-checkbox">
                        <input
                          type="checkbox"
                          checked={check.required}
                          onChange={(event) =>
                            updateCheck(index, { required: event.target.checked })
                          }
                        />
                        <span>{t('Обязательная')}</span>
                      </label>
                      <button
                        type="button"
                        className="danger ca-mini-btn"
                        onClick={() => removeCheck(index)}
                      >
                        <Trash2 size={12} aria-hidden="true" />
                        <span>{t('Удалить')}</span>
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            <div className="ca-form-row">
              <label className="ca-field">
                <span>{t('Агент-верификатор')}</span>
                <Select
                  value={draft.verifier_agent_id}
                  onChange={(value) => setDraft({ ...draft, verifier_agent_id: value })}
                  label={t('Агент-верификатор')}
                  options={[
                    { value: '', label: t('Авто (встроенный)') },
                    ...otherAgents.map((item) => ({ value: item.id, label: item.name }))
                  ]}
                />
              </label>
              <label className="ca-field">
                <span>{t('Макс. доработок')}</span>
                <input
                  type="number"
                  min={1}
                  max={10}
                  value={draft.max_rework_attempts}
                  onChange={(event) =>
                    setDraft({ ...draft, max_rework_attempts: Number(event.target.value) || 1 })
                  }
                />
              </label>
            </div>

            <label className="ca-field">
              <span>{t('Следующий агент в цепочке')}</span>
              <Select
                value={draft.next_agent_id}
                onChange={(value) => setDraft({ ...draft, next_agent_id: value })}
                label={t('Следующий агент в цепочке')}
                options={[
                  { value: '', label: t('Конец цепочки') },
                  ...otherAgents.map((item) => ({ value: item.id, label: item.name }))
                ]}
              />
            </label>
          </section>

          <section className="ca-section ca-danger-zone">
            {!confirmDelete ? (
              <button
                type="button"
                className="danger"
                onClick={() => setConfirmDelete(true)}
              >
                <Trash2 size={14} aria-hidden="true" />
                <span>{t('Удалить агента')}</span>
              </button>
            ) : (
              <div className="ca-confirm-delete">
                <span>{t('Удалить «{name}»?', { name: agent.name })}</span>
                <div className="ca-confirm-actions">
                  <button type="button" onClick={() => setConfirmDelete(false)}>
                    {t('Отмена')}
                  </button>
                  <button
                    type="button"
                    className="danger"
                    disabled={busy}
                    onClick={() => void onDelete(agent.id)}
                  >
                    {t('Подтвердить')}
                  </button>
                </div>
              </div>
            )}
          </section>
        </div>
      </form>
    </aside>
  )
}
