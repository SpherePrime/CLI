import { useState } from 'react'
import { CheckCircle2, LogOut, UserRound } from 'lucide-react'
import type { AccountState, PrimeAILimits } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import { PrimeLimitsDetails } from './PrimeLimitsDetails'

export function AccountCard({
  account,
  limits,
  onRefreshLimits,
  onLogout,
  onAfterLogout
}: {
  account: AccountState
  limits?: PrimeAILimits | null
  onRefreshLimits?: () => Promise<unknown> | void
  onLogout: () => Promise<unknown> | void
  onAfterLogout?: () => void
}) {
  const { t } = useI18n()
  const [pending, setPending] = useState(false)
  const [refreshingLimits, setRefreshingLimits] = useState(false)
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

  async function handleRefreshLimits() {
    if (!onRefreshLimits) return
    setRefreshingLimits(true)
    try {
      await onRefreshLimits()
    } finally {
      setRefreshingLimits(false)
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
      {(limits !== undefined || onRefreshLimits !== undefined) && (
        <section className="settings-card account-limits-card">
          <PrimeLimitsDetails
            limits={limits ?? null}
            onRefresh={onRefreshLimits ? handleRefreshLimits : undefined}
            refreshing={refreshingLimits}
            variant="card"
          />
        </section>
      )}
      {error && (
        <p role="alert" className="settings-status account-card-error">
          {error}
        </p>
      )}
    </>
  )
}
