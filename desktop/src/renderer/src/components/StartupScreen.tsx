import { PrimeLogo } from './PrimeLogo'
import { useI18n } from '../i18n'
import { Titlebar } from './Titlebar'
import type { ReactNode } from 'react'

export function StartupScreen({
  ready,
  failed,
  children
}: {
  ready: boolean
  failed: boolean
  children?: ReactNode
}) {
  const { t } = useI18n()
  if (ready || failed) return children ?? null
  return (
    <>
      <Titlebar />
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
    </>
  )
}
