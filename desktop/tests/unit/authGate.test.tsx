import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { AccountState, PrimeAILimits } from '../../src/shared/contracts'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { AuthGate } from '../../src/renderer/src/features/auth/AuthGate'
import { AccountCard } from '../../src/renderer/src/features/auth/AccountCard'
import {
  createNativeBridge,
  type NativeInvoke,
  type NativeListen
} from '../../src/renderer/src/host/nativeBridge'

function makeAccount(overrides: Partial<AccountState> = {}): AccountState {
  return {
    signedIn: false,
    waiting: false,
    checking: false,
    email: '',
    loginUrl: '',
    error: '',
    networkError: false,
    hasStoredSession: false,
    ...overrides
  }
}

function renderGate(account: AccountState, language: 'ru' | 'en' = 'ru') {
  return renderToStaticMarkup(
    <I18nProvider language={language}>
      <AuthGate
        account={account}
        onLogin={() => {}}
        onCancelLogin={() => {}}
        onRetryRestore={() => {}}
      />
    </I18nProvider>
  )
}

function renderAccountCard(
  account: AccountState,
  language: 'ru' | 'en' = 'ru',
  limits?: PrimeAILimits | null
) {
  return renderToStaticMarkup(
    <I18nProvider language={language}>
      <AccountCard
        account={account}
        limits={limits}
        onRefreshLimits={limits !== undefined ? () => {} : undefined}
        onLogout={() => {}}
        onAfterLogout={() => {}}
      />
    </I18nProvider>
  )
}

function makeLimits(overrides: Partial<PrimeAILimits> = {}): PrimeAILimits {
  return {
    available: true,
    planName: 'Pro',
    cacheWeightPercent: 0,
    fiveHour: {
      usedTokens: 142500,
      limitTokens: 500000,
      usedRequests: 18,
      limitRequests: 120,
      percent: 28.5,
      resetInSeconds: 3 * 3600 + 12 * 60
    },
    weekly: {
      usedTokens: 710000,
      limitTokens: 5000000,
      usedRequests: 86,
      limitRequests: 1000,
      percent: 14.2,
      resetInSeconds: 4 * 86400 + 6 * 3600
    },
    ...overrides
  }
}

describe('auth gate and account UI', () => {
  it('renders mandatory signed-out gate with 3-step rail and no skip button', () => {
    const markup = renderGate(makeAccount())
    expect(markup).toContain('PRIME ACCOUNT')
    expect(markup).toContain('Один аккаунт Prime.')
    expect(markup).toContain('Весь рабочий процесс.')
    expect(markup).toContain('Браузер Google')
    expect(markup).toContain('Подтверждение')
    expect(markup).toContain('Сессия активна')
    expect(markup).toContain('Войдите, чтобы начать')
    expect(markup).toContain('Войти через Google')
    expect(markup).toContain('role="status"')
    expect(markup).not.toContain('Пропустить')
    expect(markup).not.toContain('Skip')
  })

  it('disables sign-in while checking session', () => {
    const markup = renderGate(makeAccount({ checking: true }))
    expect(markup).toContain('Проверяем сессию…')
    expect(markup).toContain('disabled=""')
    expect(markup).toContain('auth-flow-step is-active')
  })

  it('renders waiting state with cancel, reopen and copy-link actions', () => {
    const markup = renderGate(
      makeAccount({
        waiting: true,
        loginUrl: 'https://vpn.sphereprime.ru/auth/google?app_hash=desktop_abc'
      })
    )
    expect(markup).toContain('Отменить вход')
    expect(markup).toContain('Открыть ссылку снова')
    expect(markup).toContain('Скопировать ссылку')
    expect(markup).toContain('Ожидаем подтверждение входа в браузере…')
    expect(markup).not.toContain('Войти через Google')
    expect(markup).not.toContain('disabled=""')
  })

  it('disables reopen and copy buttons when loginUrl is missing during waiting', () => {
    const markup = renderGate(makeAccount({ waiting: true, loginUrl: '' }))
    expect(markup).toContain('Открыть ссылку снова')
    expect(markup).toContain('Скопировать ссылку')
    expect(markup).toContain('disabled=""')
  })

  it('renders retry and re-login actions on network error with stored session', () => {
    const markup = renderGate(
      makeAccount({
        networkError: true,
        hasStoredSession: true,
        error: 'Сервер авторизации недоступен'
      })
    )
    expect(markup).toContain('Повторить проверку')
    expect(markup).toContain('Войти заново')
    expect(markup).toContain('role="alert"')
    expect(markup).toContain('Сервер авторизации недоступен')
    expect(markup).not.toContain('Войти через Google')
  })

  it('renders AccountCard with user email, shared CLI badge and sign-out button', () => {
    const markup = renderAccountCard(
      makeAccount({
        signedIn: true,
        email: 'user@sphereprime.ru',
        hasStoredSession: true
      })
    )
    expect(markup).toContain('user@sphereprime.ru')
    expect(markup).toContain('Единая сессия с CLI')
    expect(markup).toContain('Выйти из аккаунта')
  })

  it('translates AuthGate and AccountCard to English', () => {
    const gateMarkup = renderGate(makeAccount(), 'en')
    expect(gateMarkup).toContain('One Prime account.')
    expect(gateMarkup).toContain('Sign in to get started')
    expect(gateMarkup).toContain('Sign in with Google')
    expect(gateMarkup).not.toContain('Войти через Google')

    const cardMarkup = renderAccountCard(
      makeAccount({ signedIn: true, email: 'user@sphereprime.ru' }),
      'en'
    )
    expect(cardMarkup).toContain('Shared session with CLI')
    expect(cardMarkup).toContain('Sign out')
    expect(cardMarkup).not.toContain('Выйти из аккаунта')
  })

  it('exposes account method on native host bridge', async () => {
    const invoke = vi.fn().mockResolvedValue(makeAccount({ signedIn: true, email: 'a@b.ru' }))
    const bridge = await createNativeBridge(
      invoke as NativeInvoke,
      vi.fn().mockResolvedValue(vi.fn()) as NativeListen
    )
    await bridge.api.account('restore')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'account',
      args: ['restore']
    })
    await bridge.api.account('login')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'account',
      args: ['login']
    })
    await bridge.api.account('logout')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'account',
      args: ['logout']
    })
    await bridge.api.account('limits')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'account',
      args: ['limits']
    })
    await bridge.api.account('refreshLimits')
    expect(invoke).toHaveBeenLastCalledWith('desktop_call', {
      method: 'account',
      args: ['refreshLimits']
    })
    bridge.dispose()
  })

  it('renders PrimeAI limits section inside AccountCard in RU and EN', () => {
    const account = makeAccount({ signedIn: true, email: 'user@sphereprime.ru' })
    const ruMarkup = renderAccountCard(account, 'ru', makeLimits())
    expect(ruMarkup).toContain('PrimeAI Лимиты')
    expect(ruMarkup).toContain('Pro')
    expect(ruMarkup).toContain('5-часовой лимит')
    expect(ruMarkup).toContain('Недельный лимит')
    expect(ruMarkup).toContain('29%')
    expect(ruMarkup).toContain('14%')
    expect(ruMarkup).toContain('Сброс через 3ч 12м')
    expect(ruMarkup).toContain('Сброс через 4д 6ч')
    expect(ruMarkup).not.toContain('Кэш не расходует лимит')
    expect(ruMarkup).toContain('Обновить')

    const enMarkup = renderAccountCard(
      account,
      'en',
      makeLimits({ cacheWeightPercent: 25 })
    )
    expect(enMarkup).toContain('PrimeAI Limits')
    expect(enMarkup).toContain('5-hour limit')
    expect(enMarkup).toContain('Weekly limit')
    expect(enMarkup).toContain('Resets in 3h 12m')
    expect(enMarkup).toContain('Resets in 4d 6h')
    expect(enMarkup).not.toContain('Cache weight')
    expect(enMarkup).toContain('Refresh')
    expect(enMarkup).not.toContain('5-часовой лимит')
  })

  it('renders error fallback inside AccountCard when limits fail to load', () => {
    const markup = renderAccountCard(
      makeAccount({ signedIn: true, email: 'user@sphereprime.ru' }),
      'ru',
      {
        available: false,
        cacheWeightPercent: 0,
        error: 'Сервис лимитов временно недоступен'
      }
    )
    expect(markup).toContain('role="alert"')
    expect(markup).toContain('Сервис лимитов временно недоступен')
    expect(markup).toContain('Обновить')
  })
})
