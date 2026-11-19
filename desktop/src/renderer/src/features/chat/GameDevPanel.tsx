import { Bot, Bug, Gamepad2, Gauge, LoaderCircle } from 'lucide-react'
import { useI18n } from '../../i18n'

export function GameDevPanel({
  enabled,
  saving,
  onClose,
  onToggle
}: {
  enabled: boolean
  saving: boolean
  onClose: () => void
  onToggle: () => void | Promise<void>
}) {
  const { t } = useI18n()

  return (
    <div className="gamedev-panel">
      <header className="gamedev-panel-head">
        <span className="gamedev-panel-icon">
          <Gamepad2 size={15} aria-hidden="true" />
        </span>
        <div>
          <div className="gamedev-panel-title-row">
            <strong>{t('Режим GameDev')}</strong>
            <span className={`gamedev-status-pill ${enabled ? 'is-active' : ''}`}>
              {enabled ? t('Включён') : t('Выключен')}
            </span>
          </div>
          <small>
            {t('Добавляет в инструкции тестирования создание и запуск ботов для прохождения игры.')}
          </small>
        </div>
      </header>
      <div className="gamedev-panel-list">
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon">
            <Bot size={14} aria-hidden="true" />
          </span>
          <div>
            <strong>{t('Тестовые боты')}</strong>
            <span>{t('Автоматически проходят игровые сценарии и уровни.')}</span>
          </div>
        </div>
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon">
            <Bug size={14} aria-hidden="true" />
          </span>
          <div>
            <strong>{t('Поиск багов')}</strong>
            <span>{t('Выявляют софтлоки, краши и ошибки логики.')}</span>
          </div>
        </div>
        <div className="gamedev-panel-item">
          <span className="gamedev-panel-item-icon">
            <Gauge size={14} aria-hidden="true" />
          </span>
          <div>
            <strong>{t('Проверка сложности')}</strong>
            <span>{t('Оценивают баланс и проходимость игры.')}</span>
          </div>
        </div>
      </div>
      <p className="gamedev-panel-note">
        {t('Когда режим выключен, инструкции для игр не добавляются в промпт.')}
      </p>
      <footer className="gamedev-panel-actions">
        <button type="button" className="quiet" disabled={saving} onClick={onClose}>
          {t('Закрыть')}
        </button>
        <button
          type="button"
          className={enabled ? 'quiet danger' : 'primary'}
          disabled={saving}
          aria-busy={saving || undefined}
          onClick={() => void onToggle()}
        >
          {saving && <LoaderCircle size={13} className="spin" aria-hidden="true" />}
          {enabled ? t('Выключить') : t('Включить')}
        </button>
      </footer>
    </div>
  )
}
