import { PrimeLogo } from './PrimeLogo'
import { useI18n } from '../i18n'

export function StartupScreen({ ready, failed }: { ready: boolean; failed: boolean }) {
  const { t } = useI18n()
  if (ready || failed) return null
  return (
    <div
      className="startup-screen"
      role="status"
      aria-live="polite"
      aria-label={t('Запускаем Prime')}
    >
      <div className="startup-orbit" aria-hidden="true" />
      <div className="startup-brand">
        <div className="startup-mark">
          <PrimeLogo />
        </div>
        <h1>
          Prime<span>Desktop</span>
        </h1>
        <p>{t('ВАШИ ИДЕИ. ВАШ PRIME.')}</p>
        <div className="startup-track" aria-hidden="true">
          <span />
        </div>
        <small>
          {t('Запускаем Prime')}
          <span className="startup-dots">…</span>
        </small>
      </div>
      <span className="startup-signature">SPHEREPRIME</span>
    </div>
  )
}
