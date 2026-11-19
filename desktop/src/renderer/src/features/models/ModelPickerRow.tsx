import { Brain, Check, Lock, Star } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { ModelCapability } from './useModelCapabilities'
import type { OrchestratorRole } from './orchestrator'

function PickerIndicator({ active }: { active: boolean }) {
  return (
    <span className={`model-picker-indicator${active ? ' is-active' : ''}`} aria-hidden="true">
      {active ? <Check size={12} strokeWidth={2.5} /> : <span className="model-picker-dot" />}
    </span>
  )
}

export function OrchestratorDefaultRow({
  role,
  active,
  disabled,
  fallbackName,
  onSelect
}: {
  role: OrchestratorRole
  active: boolean
  disabled: boolean
  fallbackName?: string
  onSelect: () => void
}) {
  const { t } = useI18n()
  const baseDescription =
    role === 'planner'
      ? t('Использует модель чата / основную')
      : t('Использует быструю / основную модель')
  const detail = fallbackName ? `${baseDescription} · ${fallbackName}` : baseDescription
  return (
    <div className={`model-picker-row${active ? ' selected' : ''}`}>
      <button
        type="button"
        className="model-picker-select"
        role="option"
        aria-selected={active}
        disabled={disabled}
        onClick={() => {
          if (!disabled) onSelect()
        }}
      >
        <PickerIndicator active={active} />
        <span className="model-description">
          <span className="model-picker-title-row">
            <strong>{t('По умолчанию (авто)')}</strong>
          </span>
          <span className="model-picker-detail">
            <small>{detail}</small>
          </span>
        </span>
      </button>
    </div>
  )
}

export function ModelPickerRow({
  model,
  active,
  favorite,
  disabled,
  favoriteDisabled,
  showProvider,
  select,
  toggleFavorite
}: {
  model: ModelCapability
  active: boolean
  favorite: boolean
  disabled: boolean
  favoriteDisabled: boolean
  showProvider: boolean
  select: () => void
  toggleFavorite: () => void
}) {
  const { t } = useI18n()
  const isLocked = Boolean(model.locked)
  const planText =
    model.plan_label ||
    (isLocked && model.allowed_tariffs?.length ? model.allowed_tariffs.join(', ') : '')
  const badgeLabel =
    model.plan_label || (isLocked && model.allowed_tariffs?.length ? model.allowed_tariffs[0] : '')
  const lockTitle = planText
    ? t('Доступно в подписке: {plan}', { plan: planText })
    : t('Модель недоступна на текущем тарифе')
  const favoriteLabel = `${t(favorite ? 'Удалить из избранного' : 'Добавить в избранное')}: ${model.name || model.model_id}`
  return (
    <div
      className={`model-picker-row${active ? ' selected' : ''}${isLocked ? ' is-locked' : ''}`}
      title={isLocked ? lockTitle : undefined}
    >
      <button
        type="button"
        className="model-picker-select"
        role="option"
        aria-selected={active}
        aria-disabled={disabled || isLocked ? 'true' : undefined}
        aria-description={isLocked ? lockTitle : undefined}
        disabled={disabled || isLocked}
        title={isLocked ? lockTitle : undefined}
        onClick={() => {
          if (!disabled && !isLocked) select()
        }}
      >
        <PickerIndicator active={active} />
        <span className="model-description">
          <span className="model-picker-title-row">
            <strong>{model.name || model.model_id}</strong>
            {badgeLabel && (
              <span className={`model-plan-badge${isLocked ? ' is-locked' : ''}`}>
                {isLocked && <Lock size={11} aria-hidden="true" />}
                <span>{badgeLabel}</span>
              </span>
            )}
          </span>
          {(showProvider || model.context_window || model.can_reason) && (
            <span className="model-picker-detail">
              {showProvider && <small>{model.provider_name || model.provider_id}</small>}
              {model.context_window ? (
                <span className="model-context-pill">
                  {Math.round(model.context_window / 1000)}k
                </span>
              ) : null}
              {model.can_reason && (
                <span
                  className="model-reason-badge"
                  title={t('Поддерживает размышления')}
                  aria-hidden="true"
                >
                  <Brain size={12} />
                </span>
              )}
            </span>
          )}
        </span>
      </button>
      <button
        type="button"
        className={`model-picker-favorite${favorite ? ' active' : ''}`}
        aria-label={favoriteLabel}
        title={favoriteLabel}
        aria-pressed={favorite}
        disabled={favoriteDisabled}
        onClick={toggleFavorite}
      >
        <Star size={16} fill={favorite ? 'currentColor' : 'none'} aria-hidden="true" />
      </button>
    </div>
  )
}
