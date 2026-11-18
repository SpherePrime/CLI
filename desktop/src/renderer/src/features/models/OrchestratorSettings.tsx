import { Compass, Cpu } from 'lucide-react'
import { useI18n } from '../../i18n'
import { reasoningLabel } from './reasoning'
import type { OrchestratorRole } from './orchestrator'

export interface RoleSummary {
  name: string
  provider: string
  reasoning: string
  reasoningOptions: string[]
  hasModel: boolean
}

export function OrchestratorSettings({
  enabled,
  finalReview,
  planner,
  executor,
  disabled,
  onToggleEnabled,
  onToggleFinalReview,
  onChangeReasoning,
  onPickModel
}: {
  enabled: boolean
  finalReview: boolean
  planner: RoleSummary
  executor: RoleSummary
  disabled: boolean
  onToggleEnabled: (next: boolean) => void
  onToggleFinalReview: (next: boolean) => void
  onChangeReasoning: (role: OrchestratorRole, level: string) => void
  onPickModel: (role: OrchestratorRole) => void
}) {
  const { t } = useI18n()

  const roles: {
    id: OrchestratorRole
    title: string
    icon: typeof Compass
    summary: RoleSummary
  }[] = [
    { id: 'planner', title: t('Планировщик'), icon: Compass, summary: planner },
    { id: 'executor', title: t('Исполнитель'), icon: Cpu, summary: executor }
  ]

  return (
    <div className="orchestrator-settings">
      <div className={`orchestrator-toggle-card${enabled ? ' is-active' : ''}`}>
        <div className="orchestrator-toggle-copy">
          <strong>{t('Режим Оркестратора')}</strong>
          <p>{t('Планировщик исследует и проверяет без правок, исполнитель пишет код')}</p>
        </div>
        <button
          type="button"
          className={`orchestrator-switch${enabled ? ' checked' : ''}`}
          role="switch"
          aria-checked={enabled}
          aria-label={t('Режим Оркестратора')}
          disabled={disabled}
          onClick={() => onToggleEnabled(!enabled)}
        >
          <span className="orchestrator-switch-thumb" />
        </button>
      </div>

      <div className="orchestrator-roles">
        {roles.map(({ id, title, icon: Icon, summary }) => (
          <div className="orchestrator-role-card" key={id}>
            <div className="orchestrator-role-head">
              <div className="orchestrator-role-title">
                <Icon size={14} aria-hidden="true" />
                <span>{title}</span>
              </div>
              <button
                type="button"
                className="orchestrator-pick-btn"
                disabled={disabled}
                onClick={() => onPickModel(id)}
              >
                {t('Выбрать модель')}
              </button>
            </div>
            <div className="orchestrator-role-model">
              <strong>{summary.name || t('Модель не выбрана')}</strong>
              {summary.provider && <small>{summary.provider}</small>}
            </div>
            {summary.reasoningOptions.length > 0 && (
              <div
                className="orchestrator-reasoning-pills"
                role="group"
                aria-label={`${title}: ${t('Уровень размышлений')}`}
              >
                {summary.reasoningOptions.map((level) => {
                  const active = summary.reasoning === level
                  return (
                    <button
                      key={level}
                      type="button"
                      className={`orchestrator-pill${active ? ' active' : ''}`}
                      aria-pressed={active}
                      disabled={disabled || !summary.hasModel}
                      onClick={() => onChangeReasoning(id, level)}
                    >
                      {reasoningLabel(level, t)}
                    </button>
                  )
                })}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="orchestrator-review-row">
        <div className="orchestrator-review-copy">
          <strong id="orchestrator-final-review-label">{t('Финальная проверка плана')}</strong>
          <small>{t('Планировщик проверяет изменения перед завершением')}</small>
        </div>
        <button
          type="button"
          className={`orchestrator-switch${finalReview ? ' checked' : ''}`}
          role="switch"
          aria-checked={finalReview}
          aria-labelledby="orchestrator-final-review-label"
          disabled={disabled}
          onClick={() => onToggleFinalReview(!finalReview)}
        >
          <span className="orchestrator-switch-thumb" />
        </button>
      </div>
    </div>
  )
}
