import { Check, Star } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { ModelCapability } from './useModelCapabilities'

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
  const favoriteLabel = `${t(favorite ? 'Удалить из избранного' : 'Добавить в избранное')}: ${model.name || model.model_id}`
  return (
    <div className={`model-picker-row${active ? ' selected' : ''}`}>
      <button
        className="model-picker-select"
        role="option"
        aria-selected={active}
        disabled={disabled}
        onClick={select}
      >
        <span className="model-description">
          <strong>{model.name || model.model_id}</strong>
          <span className="model-picker-detail">
            {showProvider && <small>{model.provider_name || model.provider_id}</small>}
            {model.context_window ? (
              <small>{Math.round(model.context_window / 1000)}k</small>
            ) : null}
          </span>
        </span>
        {active && <Check size={16} aria-hidden="true" />}
      </button>
      <button
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
