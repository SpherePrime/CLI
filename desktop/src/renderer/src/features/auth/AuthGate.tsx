import { useEffect, useRef, useState } from 'react'
import { AlertCircle, Check, Copy, ExternalLink, RefreshCw, X } from 'lucide-react'
import type { AccountState } from '../../../../shared/contracts'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useI18n } from '../../i18n'

export type AuthGateProps = {
  account: AccountState
  onLogin: () => Promise<unknown> | void
  onCancelLogin: () => Promise<unknown> | void
  onRetryRestore: () => Promise<unknown> | void
}

type StepState = 'done' | 'active' | 'error' | 'pending'

function GoogleMark() {
  return (
    <svg className="auth-google-mark" viewBox="0 0 18 18" aria-hidden="true">
      <path
        fill="currentColor"
        d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.48h4.84a4.14 4.14 0 0 1-1.8 2.72v2.26h2.92c1.71-1.57 2.68-3.89 2.68-6.62Z"
      />
      <path
        fill="currentColor"
        d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.92-2.26c-.8.54-1.83.86-3.04.86-2.34 0-4.32-1.58-5.03-3.71H.96v2.33A9 9 0 0 0 9 18Z"
      />
      <path
        fill="currentColor"
        d="M3.97 10.71A5.41 5.41 0 0 1 3.69 9c0-.59.1-1.17.28-1.71V4.96H.96A9 9 0 0 0 0 9c0 1.45.35 2.82.96 4.04l3.01-2.33Z"
      />
      <path
        fill="currentColor"
        d="M9 3.58c1.32 0 2.51.45 3.44 1.35l2.58-2.59C13.46.89 11.43 0 9 0A9 9 0 0 0 .96 4.96l3.01 2.33C4.68 5.16 6.66 3.58 9 3.58Z"
      />
    </svg>
  )
}

export function AuthGate({ account, onLogin, onCancelLogin, onRetryRestore }: AuthGateProps) {
  const { t } = useI18n()
  const [copyStatus, setCopyStatus] = useState<'idle' | 'copied' | 'error'>('idle')
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (copyTimer.current) clearTimeout(copyTimer.current)
    }
  }, [])

  useEffect(() => {
    if (!account.waiting) setCopyStatus('idle')
  }, [account.waiting])

  async function handleCopyLink() {
    if (!account.loginUrl) return
    if (copyTimer.current) clearTimeout(copyTimer.current)
    try {
      await navigator.clipboard.writeText(account.loginUrl)
      setCopyStatus('copied')
    } catch {
      setCopyStatus('error')
    }
    copyTimer.current = setTimeout(() => {
      setCopyStatus('idle')
    }, 1800)
  }

  const hasError = Boolean(account.error || account.networkError)
  const isBusy = Boolean(account.waiting || account.checking)
  const stepOneState: StepState =
    account.signedIn || account.waiting || account.checking || account.hasStoredSession
      ? 'done'
      : hasError
        ? 'error'
        : 'active'
  const stepTwoState: StepState = account.signedIn
    ? 'done'
    : account.waiting || account.checking
      ? 'active'
      : hasError
        ? 'error'
        : 'pending'
  const stepThreeState: StepState = account.signedIn ? 'done' : 'pending'

  const steps: { index: string; label: string; detail: string; state: StepState }[] = [
    {
      index: '01',
      label: t('Браузер Google'),
      detail: account.waiting
        ? t('Страница входа открыта в браузере')
        : t('Откроется окно входа Google'),
      state: stepOneState
    },
    {
      index: '02',
      label: t('Подтверждение'),
      detail: account.checking
        ? t('Проверка сохранённой сессии')
        : hasError && !account.waiting
          ? t('Требуется повторная проверка')
          : t('Ожидаем подтверждение в браузере'),
      state: stepTwoState
    },
    {
      index: '03',
      label: t('Сессия активна'),
      detail: account.signedIn ? t('Вход подтверждён') : t('Доступ к Desktop и CLI'),
      state: stepThreeState
    }
  ]

  const showNetworkRetry = !account.waiting && account.networkError && account.hasStoredSession
  const errorMessage =
    account.error ||
    (account.networkError
      ? t('Не удалось проверить сессию из-за ошибки сети. Повторите проверку или войдите заново.')
      : '')

  const liveStatus =
    copyStatus === 'copied'
      ? t('Ссылка скопирована')
      : copyStatus === 'error'
        ? t('Не удалось скопировать ссылку')
        : account.waiting
          ? t('Ожидаем подтверждение входа в браузере…')
          : account.checking
            ? t('Проверяем сессию…')
            : showNetworkRetry
              ? t('Сессия не подтверждена — проверьте соединение.')
              : t('Для продолжения подтвердите вход через Google.')

  return (
    <main
      className={`auth-gate${isBusy ? ' is-busy' : ''}${hasError ? ' is-error' : ''}`}
      aria-label={t('Вход в аккаунт Prime')}
    >
      <div className="auth-gate-backdrop" aria-hidden="true">
        <div className="auth-gate-grid" />
        <div className="auth-gate-orb auth-gate-orb-copper" />
        <div className="auth-gate-orb auth-gate-orb-slate" />
      </div>

      <div className="auth-gate-stage">
        <section className="auth-gate-editorial">
          <div className="auth-gate-brand-row">
            <span className="auth-gate-mark" aria-hidden="true">
              <span className="auth-gate-mark-aura" />
              <span className="auth-gate-ring auth-gate-ring-outer" />
              <span className="auth-gate-ring auth-gate-ring-inner" />
              <span className="auth-gate-mark-core">
                <PrimeLogo />
              </span>
            </span>
            <span className="auth-gate-eyebrow">PRIME ACCOUNT</span>
          </div>

          <h1 className="auth-gate-heading">
            <span>{t('Один аккаунт Prime.')}</span>
            <span className="auth-gate-heading-accent">{t('Весь рабочий процесс.')}</span>
          </h1>

          <p className="auth-gate-lead">
            {t('Авторизация синхронизируется с Prime CLI на этом устройстве.')}
          </p>

          <ol
            className={`auth-flow-rail${isBusy ? ' is-busy' : ''}`}
            aria-label={t('Этапы входа')}
          >
            {steps.map((step) => (
              <li key={step.index} className={`auth-flow-step is-${step.state}`}>
                <span className="auth-flow-node" aria-hidden="true">
                  {step.state === 'done' ? <Check size={12} /> : step.index}
                </span>
                <div className="auth-flow-copy">
                  <strong>{step.label}</strong>
                  <span>{step.detail}</span>
                </div>
              </li>
            ))}
          </ol>
        </section>

        <section
          className={`auth-gate-card${isBusy ? ' is-busy' : ''}${hasError ? ' is-error' : ''}`}
          aria-labelledby="auth-gate-card-title"
        >
          <div className="auth-gate-card-head">
            <h2 id="auth-gate-card-title">{t('Войдите, чтобы начать')}</h2>
            <p>{t('Используйте аккаунт Google, чтобы открыть рабочее пространство Prime.')}</p>
          </div>

          <div className="auth-gate-actions">
            {account.waiting ? (
              <>
                <div className="auth-gate-wait-beacon" aria-hidden="true">
                  <span className="auth-gate-wait-badge">
                    <span className="auth-gate-wait-ring" />
                    <GoogleMark />
                  </span>
                  <span className="auth-gate-wait-track">
                    <span className="auth-gate-wait-sweep" />
                  </span>
                </div>
                <button
                  type="button"
                  className="auth-gate-cancel"
                  onClick={() => void onCancelLogin()}
                >
                  <X size={16} aria-hidden="true" />
                  <span>{t('Отменить вход')}</span>
                </button>
                <div className="auth-gate-secondary-row">
                  <button
                    type="button"
                    className="auth-gate-secondary"
                    disabled={!account.loginUrl}
                    onClick={() => {
                      if (account.loginUrl) void window.prime.openExternal(account.loginUrl)
                    }}
                  >
                    <ExternalLink size={15} aria-hidden="true" />
                    <span>{t('Открыть ссылку снова')}</span>
                  </button>
                  <button
                    type="button"
                    className="auth-gate-secondary"
                    disabled={!account.loginUrl}
                    onClick={() => void handleCopyLink()}
                  >
                    {copyStatus === 'copied' ? (
                      <Check size={15} aria-hidden="true" />
                    ) : (
                      <Copy size={15} aria-hidden="true" />
                    )}
                    <span>
                      {copyStatus === 'copied' ? t('Ссылка скопирована') : t('Скопировать ссылку')}
                    </span>
                  </button>
                </div>
              </>
            ) : showNetworkRetry ? (
              <>
                <button
                  type="button"
                  className="primary auth-gate-primary"
                  disabled={account.checking}
                  onClick={() => void onRetryRestore()}
                >
                  <RefreshCw
                    size={16}
                    className={account.checking ? 'spin' : undefined}
                    aria-hidden="true"
                  />
                  <span>
                    {account.checking ? t('Проверяем сессию…') : t('Повторить проверку')}
                  </span>
                </button>
                <button
                  type="button"
                  className="auth-gate-secondary auth-gate-secondary-full"
                  disabled={account.checking}
                  onClick={() => void onLogin()}
                >
                  <GoogleMark />
                  <span>{t('Войти заново')}</span>
                </button>
              </>
            ) : (
              <button
                type="button"
                className="primary auth-gate-primary"
                disabled={account.checking}
                onClick={() => void onLogin()}
              >
                {account.checking ? (
                  <RefreshCw size={16} className="spin" aria-hidden="true" />
                ) : (
                  <GoogleMark />
                )}
                <span>
                  {account.checking ? t('Проверяем сессию…') : t('Войти через Google')}
                </span>
              </button>
            )}
          </div>

          <p className="auth-gate-status" role="status" aria-live="polite">
            <span
              className={`auth-gate-status-dot ${
                isBusy ? 'is-busy' : hasError ? 'is-error' : ''
              }`}
              aria-hidden="true"
            />
            <span>{liveStatus}</span>
          </p>

          {errorMessage && (
            <div className="auth-gate-alert" role="alert">
              <AlertCircle size={16} aria-hidden="true" />
              <span>{errorMessage}</span>
            </div>
          )}
        </section>
      </div>
    </main>
  )
}
