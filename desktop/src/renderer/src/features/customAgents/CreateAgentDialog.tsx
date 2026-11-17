import { useRef, useState } from 'react'
import { Bot, Sparkles, Wrench } from 'lucide-react'
import type {
  CustomAgent,
  CustomAgentModelMode,
  CustomAgentRole,
  CustomAgentTriggerType
} from '../../../../shared/customAgents'
import { Modal } from '../../components/Modal'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { AGENT_PRESETS, type AgentPreset } from './presentation'

export function CreateAgentDialog({
  agents,
  workspacePath,
  onCreate,
  onDispatchOrchestrator,
  close
}: {
  agents: CustomAgent[]
  workspacePath: string
  onCreate: (input: Partial<CustomAgent>) => Promise<CustomAgent>
  onDispatchOrchestrator: (orchestratorId: string, prompt: string) => Promise<void>
  close: () => void
}) {
  const { t } = useI18n()
  const [mode, setMode] = useState<'manual' | 'orchestrator'>('manual')
  const [presetId, setPresetId] = useState<string>('researcher')
  const [name, setName] = useState(AGENT_PRESETS[1].name)
  const [role, setRole] = useState<CustomAgentRole>(AGENT_PRESETS[1].role)
  const [description, setDescription] = useState(AGENT_PRESETS[1].description)
  const [triggerType, setTriggerType] = useState<CustomAgentTriggerType>(AGENT_PRESETS[1].trigger_type)
  const [modelMode, setModelMode] = useState<CustomAgentModelMode>('auto')
  const [systemPrompt, setSystemPrompt] = useState(AGENT_PRESETS[1].system_prompt)
  const [bindWorkspace, setBindWorkspace] = useState(Boolean(workspacePath))
  const [orchestratorPrompt, setOrchestratorPrompt] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const nameRef = useRef<HTMLInputElement>(null)

  const orchestrator = agents.find((a) => a.role === 'orchestrator' && a.enabled) ||
    agents.find((a) => a.role === 'orchestrator')

  const applyPreset = (preset: AgentPreset) => {
    setPresetId(preset.id)
    setName(preset.name)
    setRole(preset.role)
    setDescription(preset.description)
    setTriggerType(preset.trigger_type)
    setSystemPrompt(preset.system_prompt)
  }

  const handleSubmitManual = async (event: React.FormEvent) => {
    event.preventDefault()
    const trimmedName = name.trim()
    if (!trimmedName) {
      setError(t('Укажите название агента'))
      return
    }
    const preset = AGENT_PRESETS.find((p) => p.id === presetId)
    setSubmitting(true)
    setError('')
    try {
      await onCreate({
        name: trimmedName,
        role,
        description: description.trim(),
        trigger_type: triggerType,
        interval_minutes: preset?.interval_minutes || (triggerType === 'monitor' ? 15 : 0),
        monitor_kind: preset?.monitor_kind || (triggerType === 'monitor' ? 'http' : ''),
        monitor_target: preset?.monitor_target || '',
        model_mode: modelMode,
        auto_model_hint: preset?.auto_model_hint || '',
        system_prompt: systemPrompt.trim(),
        acceptance_criteria: preset?.acceptance_criteria || '',
        deterministic_checks: preset?.deterministic_checks || [],
        workspace_path: bindWorkspace ? workspacePath : '',
        enabled: true
      })
      close()
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setSubmitting(false)
    }
  }

  const handleSubmitOrchestrator = async (event: React.FormEvent) => {
    event.preventDefault()
    const task = orchestratorPrompt.trim()
    if (!task) {
      setError(t('Опишите задачу для нового агента'))
      return
    }
    if (!orchestrator) {
      setError(t('Сначала создайте агента с ролью «Координатор»'))
      return
    }
    setSubmitting(true)
    setError('')
    try {
      await onDispatchOrchestrator(
        orchestrator.id,
        `Спроектируй и создай нового агента под задачу: ${task}`
      )
      close()
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      title={t('Новый агент')}
      close={() => {
        if (!submitting) close()
      }}
      wide
      className="ca-create-modal"
      initialFocus={nameRef}
    >
      <div className="ca-create-body">
        <div className="ca-segmented" role="tablist" aria-label={t('Способ создания')}>
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'manual'}
            className={mode === 'manual' ? 'active' : ''}
            onClick={() => {
              setMode('manual')
              setError('')
            }}
          >
            <Wrench size={14} aria-hidden="true" />
            <span>{t('Вручную')}</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === 'orchestrator'}
            className={mode === 'orchestrator' ? 'active' : ''}
            onClick={() => {
              setMode('orchestrator')
              setError('')
            }}
          >
            <Sparkles size={14} aria-hidden="true" />
            <span>{t('Через координатора')}</span>
          </button>
        </div>

        {mode === 'manual' ? (
          <form className="ca-form" onSubmit={handleSubmitManual}>
            <div className="ca-presets-grid" role="group" aria-label={t('Шаблоны агентов')}>
              {AGENT_PRESETS.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className={`ca-preset-card ${presetId === preset.id ? 'active' : ''}`}
                  aria-pressed={presetId === preset.id}
                  onClick={() => applyPreset(preset)}
                >
                  <strong>{t(preset.label)}</strong>
                  <span>{t(preset.description)}</span>
                </button>
              ))}
            </div>

            <div className="ca-form-row">
              <label className="ca-field">
                <span>{t('Название')}</span>
                <input
                  ref={nameRef}
                  type="text"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={t('Например: Аудитор сборок')}
                />
              </label>

              <label className="ca-field">
                <span>{t('Роль')}</span>
                <Select
                  value={role}
                  onChange={(value) => setRole(value as CustomAgentRole)}
                  label={t('Роль')}
                  options={[
                    { value: 'worker', label: t('Воркер') },
                    { value: 'orchestrator', label: t('Координатор') },
                    { value: 'verifier', label: t('Верификатор') }
                  ]}
                />
              </label>
            </div>

            <div className="ca-form-row">
              <label className="ca-field">
                <span>{t('Триггер')}</span>
                <Select
                  value={triggerType}
                  onChange={(value) => setTriggerType(value as CustomAgentTriggerType)}
                  label={t('Триггер')}
                  options={[
                    { value: 'manual', label: t('Вручную') },
                    { value: 'monitor', label: t('Мониторинг') },
                    { value: 'interval', label: t('Интервал') },
                    { value: 'cron', label: t('Cron') },
                    { value: 'event', label: t('Событие') }
                  ]}
                />
              </label>

              <label className="ca-field">
                <span>{t('Выбор модели')}</span>
                <Select
                  value={modelMode}
                  onChange={(value) => setModelMode(value as CustomAgentModelMode)}
                  label={t('Выбор модели')}
                  options={[
                    { value: 'auto', label: t('Авто-подбор ИИ') },
                    { value: 'manual', label: t('Вручную') }
                  ]}
                />
              </label>
            </div>

            <label className="ca-field">
              <span>{t('Описание')}</span>
              <input
                type="text"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder={t('Краткая цель агента')}
              />
            </label>

            <label className="ca-field">
              <span>{t('Системный промпт')}</span>
              <textarea
                rows={4}
                value={systemPrompt}
                onChange={(event) => setSystemPrompt(event.target.value)}
                placeholder={t('Инструкция поведения и формат ответа')}
              />
            </label>

            {workspacePath && (
              <label className="ca-checkbox">
                <input
                  type="checkbox"
                  checked={bindWorkspace}
                  onChange={(event) => setBindWorkspace(event.target.checked)}
                />
                <span>{t('Привязать к проекту: {path}', { path: workspacePath })}</span>
              </label>
            )}

            {error && <p className="ca-error" role="alert">{error}</p>}

            <div className="ca-modal-actions">
              <button type="button" onClick={close} disabled={submitting}>
                {t('Отмена')}
              </button>
              <button type="submit" className="primary" disabled={submitting}>
                <Bot size={15} aria-hidden="true" />
                <span>{submitting ? t('Создаём…') : t('Создать агента')}</span>
              </button>
            </div>
          </form>
        ) : (
          <form className="ca-form" onSubmit={handleSubmitOrchestrator}>
            <p className="ca-help">
              {t('Опишите задачу в 1–2 предложениях. Координатор спроектирует агента, настроит проверки и подберёт уровень модели.')}
            </p>
            <label className="ca-field">
              <span>{t('Задача для нового агента')}</span>
              <textarea
                rows={4}
                value={orchestratorPrompt}
                onChange={(event) => setOrchestratorPrompt(event.target.value)}
                placeholder={t('Например: Проверяй каждые 15 минут RSS ленту релизов и присылай краткую сводку изменений.')}
              />
            </label>
            <div className="ca-info-banner">
              {orchestrator
                ? t('Исполнитель проектирования: {name}', { name: orchestrator.name })
                : t('Координатор не найден — создайте его вручную во вкладке «Вручную».')}
            </div>

            {error && <p className="ca-error" role="alert">{error}</p>}

            <div className="ca-modal-actions">
              <button type="button" onClick={close} disabled={submitting}>
                {t('Отмена')}
              </button>
              <button
                type="submit"
                className="primary"
                disabled={submitting || !orchestrator}
              >
                <Sparkles size={15} aria-hidden="true" />
                <span>{submitting ? t('Отправляем…') : t('Поручить координатору')}</span>
              </button>
            </div>
          </form>
        )}
      </div>
    </Modal>
  )
}
