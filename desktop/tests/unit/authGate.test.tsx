import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { AccountState } from '../../src/shared/contracts'
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

function renderAccountCard(account: AccountState, language: 'ru' | 'en' = 'ru') {
  return renderToStaticMarkup(
    <I18nProvider language={language}>
      <AccountCard account={account} onLogout={() => {}} onAfterLogout={() => {}} />
    </I18nProvider>
  )
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
    bridge.dispose()
  })
})
