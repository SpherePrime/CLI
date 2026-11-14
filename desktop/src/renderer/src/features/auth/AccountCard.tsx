import { useState } from 'react'
import { CheckCircle2, LogOut, UserRound } from 'lucide-react'
import type { AccountState } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'

export function AccountCard({
  account,
  onLogout,
  onAfterLogout
}: {
  account: AccountState
  onLogout: () => Promise<unknown> | void
  onAfterLogout?: () => void
}) {
  const { t } = useI18n()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  async function handleLogout() {
    setPending(true)
    setError('')
    try {
      await onLogout()
      onAfterLogout?.()
    } catch (err) {
      setError(String((err as Error)?.message || err))
    } finally {
      setPending(false)
    }
  }

  return (
    <>
      <h3>{t('Аккаунт Prime')}</h3>
      <p className="muted">
        {t('Единая авторизация для Prime Desktop и Prime CLI на этом устройстве.')}
      </p>
      <section className="settings-card account-card">
        <div className="account-card-identity">
          <span className="account-card-avatar" aria-hidden="true">
            <UserRound size={18} />
          </span>
          <div className="account-card-meta">
            <span className="account-card-label">{t('Текущий аккаунт')}</span>
            <strong className="account-card-email">
              {account.email || t('Аккаунт подключён')}
            </strong>
          </div>
          <span className="account-card-badge">
            <CheckCircle2 size={13} aria-hidden="true" />
            <span>{t('Единая сессия с CLI')}</span>
          </span>
        </div>
        <div className="settings-actions">
          <button
            type="button"
            className="account-card-logout"
            disabled={pending}
            onClick={() => void handleLogout()}
          >
            <LogOut size={15} aria-hidden="true" />
            <span>{pending ? t('Выходим…') : t('Выйти из аккаунта')}</span>
          </button>
        </div>
      </section>
      {error && (
        <p role="alert" className="settings-status account-card-error">
          {error}
        </p>
      )}
    </>
  )
}
