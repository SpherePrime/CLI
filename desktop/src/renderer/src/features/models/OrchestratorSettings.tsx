import { ChevronRight, Compass, Cpu } from 'lucide-react'
import { useI18n } from '../../i18n'
import { reasoningLabel } from './reasoning'
import { compactReasoningLabel, type OrchestratorRole } from './orchestrator'

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
      <button
        type="button"
        className={`orchestrator-toggle-row${enabled ? ' is-active' : ''}`}
        role="switch"
        aria-checked={enabled}
        disabled={disabled}
        onClick={() => onToggleEnabled(!enabled)}
      >
        <span className="orchestrator-toggle-copy">
          <strong>{t('Режим Оркестратора')}</strong>
          <span>{t('Планировщик проверяет, исполнитель пишет код')}</span>
        </span>
        <span className={`orchestrator-switch${enabled ? ' checked' : ''}`} aria-hidden="true">
          <span className="orchestrator-switch-thumb" />
        </span>
      </button>

      {roles.map(({ id, title, icon: Icon, summary }) => (
        <div className="orchestrator-role-section" key={id}>
          <button
            type="button"
            className="orchestrator-role-trigger"
            disabled={disabled}
            onClick={() => onPickModel(id)}
          >
            <span className="orchestrator-role-kicker">
              <Icon size={13} aria-hidden="true" />
              <span>{title}</span>
            </span>
            <span className="orchestrator-role-main">
              <span className="orchestrator-role-model">
                <strong>{summary.name || t('Модель не выбрана')}</strong>
                {summary.provider && <small>{summary.provider}</small>}
              </span>
              <ChevronRight size={14} className="orchestrator-role-chevron" aria-hidden="true" />
            </span>
          </button>

          {summary.reasoningOptions.length > 0 && (
            <div
              className="segmented orchestrator-reasoning-segmented"
              role="group"
              aria-label={`${title}: ${t('Уровень размышлений')}`}
            >
              {summary.reasoningOptions.map((level) => {
                const active = summary.reasoning === level
                const fullLabel = reasoningLabel(level, t)
                return (
                  <button
                    key={level}
                    type="button"
                    className={active ? 'active' : ''}
                    aria-label={fullLabel}
                    aria-pressed={active}
                    title={fullLabel}
                    disabled={disabled || !summary.hasModel}
                    onClick={() => onChangeReasoning(id, level)}
                  >
                    {compactReasoningLabel(level, t)}
                  </button>
                )
              })}
            </div>
          )}
        </div>
      ))}

      <div className="orchestrator-review-section">
        <button
          type="button"
          className="orchestrator-review-row"
          role="switch"
          aria-checked={finalReview}
          disabled={disabled}
          onClick={() => onToggleFinalReview(!finalReview)}
        >
          <span className="orchestrator-review-copy">
            <strong>{t('Финальная проверка')}</strong>
            <span>{t('перед ответом')}</span>
          </span>
          <span className={`orchestrator-switch${finalReview ? ' checked' : ''}`} aria-hidden="true">
            <span className="orchestrator-switch-thumb" />
          </span>
        </button>
      </div>
    </div>
  )
}
