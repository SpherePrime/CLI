import { Bot, Bug, Gamepad2, Gauge, LoaderCircle } from 'lucide-react'
import { useI18n } from '../../i18n'

export function GameDevPanel({
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
    <div className="gamedev-panel">
      <div className="gamedev-panel-head">
        <span className={`gamedev-panel-icon${enabled ? ' is-active' : ''}`} aria-hidden="true">
          <Gamepad2 size={16} />
        </span>
        <div className="gamedev-panel-head-text">
          <div className="gamedev-panel-title-row">
            <strong>{t('Режим GameDev')}</strong>
            <span className={`gamedev-status-pill${enabled ? ' is-active' : ''}`}>
              {enabled && <span className="gamedev-status-dot" aria-hidden="true" />}
              {enabled ? t('Включён') : t('Выключен')}
            </span>
          </div>
          <small>
            {t('Запускает живого бота-тестера, проверяет визуал и баланс игры.')}
          </small>
        </div>
      </div>

      <div className="gamedev-panel-list">
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon" aria-hidden="true">
            <Bot size={14} />
          </span>
          <div>
            <strong>{t('Живой бот-тестер')}</strong>
            <span>{t('Проходит игру и ищет баги, углы и софтлоки.')}</span>
          </div>
        </div>
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon" aria-hidden="true">
            <Bug size={14} />
          </span>
          <div>
            <strong>{t('Визуал и провалы в текстуры')}</strong>
            <span>{t('Проверяет коллизии, рендер, ассеты и FPS.')}</span>
          </div>
        </div>
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon" aria-hidden="true">
            <Gauge size={14} />
          </span>
          <div>
            <strong>{t('Баланс сложности')}</strong>
            <span>{t('Оценивает тайминги и проходимость.')}</span>
          </div>
        </div>
      </div>

      <div className="gamedev-panel-actions">
        <button type="button" className="gamedev-panel-close" onClick={onClose}>
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
