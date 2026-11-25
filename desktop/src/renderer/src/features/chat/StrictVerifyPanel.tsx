import { BadgeCheck, LoaderCircle, RefreshCw, UserCheck } from 'lucide-react'
import { useI18n } from '../../i18n'

export function StrictVerifyPanel({
  enabled,
  saving,
  onToggle,
  onClose
}: {
  enabled: boolean
  saving: boolean
  onToggle: () => void
  onClose: () => void
}) {
  const { t } = useI18n()

  return (
    <div className="strict-verify-panel">
      <div className="strict-verify-panel-head">
        <span
          className={`strict-verify-panel-icon${enabled ? ' is-active' : ''}`}
          aria-hidden="true"
        >
          <BadgeCheck size={16} />
        </span>
        <div className="strict-verify-panel-head-text">
          <div className="strict-verify-panel-title-row">
            <strong>{t('Строгая проверка')}</strong>
            <span className={`strict-verify-status-pill${enabled ? ' is-active' : ''}`}>
              {enabled && <span className="strict-verify-status-dot" aria-hidden="true" />}
              {enabled ? t('Включена') : t('Выключена')}
            </span>
          </div>
          <small>{t('Обязательный независимый ревьюер проверяет результат до идеала.')}</small>
        </div>
      </div>

      <div className="strict-verify-panel-list">
        <div className="strict-verify-panel-item">
          <span className="strict-verify-panel-item-icon" aria-hidden="true">
            <UserCheck size={14} />
          </span>
          <div>
            <strong>{t('Независимый ревьюер')}</strong>
            <span>{t('Отдельный агент со свежим контекстом проверяет код, логику и поведение.')}</span>
          </div>
        </div>
        <div className="strict-verify-panel-item">
          <span className="strict-verify-panel-item-icon" aria-hidden="true">
            <RefreshCw size={14} />
          </span>
          <div>
            <strong>{t('Цикл до идеала')}</strong>
            <span>{t('Крутит правки и ре-ревью до APPROVED с защитой от зацикливания.')}</span>
          </div>
        </div>
      </div>

      <div className="strict-verify-panel-actions">
        <button type="button" className="strict-verify-panel-close" onClick={onClose}>
          {t('Закрыть')}
        </button>
        <button
          type="button"
          className={enabled ? 'quiet danger' : 'primary'}
          disabled={saving}
          onClick={onToggle}
        >
          {saving && <LoaderCircle size={13} className="spin" aria-hidden="true" />}
          {enabled ? t('Выключить') : t('Включить')}
        </button>
      </div>
    </div>
  )
}
