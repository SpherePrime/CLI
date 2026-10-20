import { EmptyState } from '../../components/EmptyState'
import { useEffect, useRef, useState } from 'react'
import {
  ArrowUpRight,
  Cable,
  Circle,
  Pencil,
  Plus,
  Server,
  ShieldAlert,
  Trash2,
  Unplug
} from 'lucide-react'
import type { DesktopEvent } from '../../../../shared/contracts'
import type {
  RemoteApi,
  RemoteCredentials,
  RemoteProfileState,
  RemoteRequest,
  RemoteSnapshot
} from '../../../../shared/remote'
import { useI18n } from '../../i18n'
import { RemoteProfileEditor } from './RemoteProfileEditor'
import { remoteError, remoteProgress, statusLabel } from './presentation'
import '../../styles/remote.css'
type Props = {
  api: RemoteApi
  subscribe(listener: (event: DesktopEvent) => void): () => void
  onError(message: string): void
  onOpenProject(profileId: string): void
  chooseKey?(): Promise<string | undefined>
}
export function RemotePanel({ api, subscribe, onError, onOpenProject, chooseKey }: Props) {
  const { t } = useI18n()
  const [snapshot, setSnapshot] = useState<RemoteSnapshot>({ profiles: [] })
  const [editor, setEditor] = useState<RemoteProfileState | 'new'>()
  const [credentials, setCredentials] = useState<Record<string, RemoteCredentials>>({})
  const [confirm, setConfirm] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const mounted = useRef(true)
  const version = useRef(0)
  const pending = useRef(false)
  const request = async (operation: RemoteRequest) => {
    if (pending.current) return false
    pending.current = true
    setBusy(true)
    setError('')
    const current = ++version.current
    try {
      const result = await api(operation)
      if (mounted.current && current === version.current) setSnapshot(result)
      if (result.error) throw new Error(result.error)
      return true
    } catch (failure) {
      if (mounted.current) {
        const message = remoteError(failure, t)
        setError(message)
        onError(message)
      }
      return false
    } finally {
      pending.current = false
      if (mounted.current) setBusy(false)
    }
  }
  useEffect(() => {
    mounted.current = true
    const current = ++version.current
    void api({ action: 'list' })
      .then((result) => {
        if (mounted.current && current === version.current) setSnapshot(result)
      })
      .catch((failure) => {
        if (mounted.current) setError(remoteError(failure, t))
      })
    const unsubscribe = subscribe((event) => {
      if (event.type === 'remote') {
        version.current++
        setSnapshot(event.payload as RemoteSnapshot)
      }
    })
    return () => {
      mounted.current = false
      unsubscribe()
    }
  }, [api, subscribe])
  const readOnly = Boolean(snapshot.error)
  const connect = async (profile: RemoteProfileState) => {
    const secret = credentials[profile.id]
    setCredentials((current) => {
      const next = { ...current }
      delete next[profile.id]
      return next
    })
    await request({ action: 'connect', id: profile.id, credentials: secret })
  }
  return (
    <section className="remote-panel" aria-label={t('Удалённые проекты')}>
      <header className="remote-header">
        <div className="remote-header-icon">
          <Server size={19} />
        </div>
        <div>
          <h3>{t('Удалённые проекты')}</h3>
          <p>{t('Рабочее пространство через SSH')}</p>
        </div>
        <button
          className="icon-button"
          aria-label={t('Добавить сервер')}
          disabled={busy || readOnly}
          onClick={() => setEditor('new')}
        >
          <Plus size={18} />
        </button>
      </header>
      <p className="remote-intro">
        {t('Файлы, Git, терминал и Prime работают на вашем Linux-сервере.')}
      </p>
      {(error || snapshot.error || snapshot.credentialError) && (
        <div role="alert" className="remote-error">
          {error || remoteError(snapshot.error || snapshot.credentialError, t)}
        </div>
      )}
      {editor && (
        <RemoteProfileEditor
          key={editor === 'new' ? 'new' : editor.id}
          initial={editor === 'new' ? undefined : editor}
          busy={busy}
          onSave={(input) => request({ action: 'save', input })}
          onClose={() => setEditor(undefined)}
          chooseKey={chooseKey}
        />
      )}
      {!snapshot.profiles.length && !editor && (
        <EmptyState
          icon={Cable}
          title={t('Подключите первый сервер')}
          text={t('Укажите адрес и папку проекта. Отпечаток сервера проверяется перед входом.')}
          action={{
            label: t('Добавить сервер'),
            icon: Plus,
            onClick: () => setEditor('new'),
            disabled: busy || readOnly
          }}
        />
      )}
      <div className="remote-list">
        {snapshot.profiles.map((profile) => {
          const working = ['connecting', 'preparing', 'disconnecting'].includes(profile.status)
          const secret = credentials[profile.id] ?? {}
          return (
            <article className={`remote-card is-${profile.status}`} key={profile.id}>
              <div className="remote-card-heading">
                <div className="remote-server-icon">
                  <Server size={18} />
                </div>
                <div className="remote-card-title">
                  <strong>{profile.name}</strong>
                  <span>
                    {profile.user}@{profile.host}:{profile.port}
                  </span>
                </div>
                <span className="remote-status">
                  <Circle size={7} fill="currentColor" />
                  {t(statusLabel(profile.status))}
                </span>
              </div>
              <div className="remote-root" title={profile.root}>
                {profile.root}
              </div>
              {profile.error && (
                <p role="alert" className="remote-error">
                  {remoteError(profile.error, t)}
                </p>
              )}
              {profile.progress && (
                <div role="status" className="remote-progress">
                  <span>{remoteProgress(profile.progress.message, t)}</span>
                  {profile.progress.percent !== undefined && (
                    <progress max="100" value={profile.progress.percent} />
                  )}
                </div>
              )}
              {profile.challenge && (
                <div className="remote-trust">
                  <ShieldAlert size={19} />
                  <strong>
                    {t(profile.challenge.previous ? 'Ключ сервера изменился' : 'Проверьте сервер')}
                  </strong>
                  <p>
                    {t(
                      profile.challenge.previous
                        ? 'Подключение остановлено. Сверьте новый отпечаток с администратором сервера.'
                        : 'Сверьте отпечаток с администратором перед первым подключением.'
                    )}
                  </p>
                  {profile.challenge.previous && (
                    <>
                      <span>{t('Предыдущий отпечаток')}</span>
                      <code>{profile.challenge.previous}</code>
                    </>
                  )}
                  <span>{t('Отпечаток сервера')}</span>
                  <code>{profile.challenge.fingerprint}</code>
                  <button
                    disabled={busy || readOnly}
                    onClick={() => {
                      if (confirm === profile.id) {
                        setConfirm('')
                        void request({
                          action: 'trust',
                          id: profile.id,
                          fingerprint: profile.challenge!.fingerprint
                        })
                      } else setConfirm(profile.id)
                    }}
                  >
                    {t(confirm === profile.id ? 'Подтвердить доверие' : 'Я проверил отпечаток')}
                  </button>
                  {confirm === profile.id && (
                    <p>
                      {t('Доверие сохранит этот ключ. После подтверждения подключитесь снова.')}
                    </p>
                  )}
                </div>
              )}
              {!working &&
                profile.status !== 'connected' &&
                !profile.challenge &&
                profile.auth !== 'agent' && (
                  <label className="remote-secret">
                    {t(profile.auth === 'key' ? 'Пароль ключа (если задан)' : 'Пароль')}
                    <input
                      type="password"
                      autoComplete="off"
                      placeholder={
                        profile.hasSavedCredentials ? t('Сохранённый пароль') : undefined
                      }
                      value={(profile.auth === 'key' ? secret.passphrase : secret.password) ?? ''}
                      onChange={(event) => {
                        const value = event.target.value
                        setCredentials((current) => ({
                          ...current,
                          [profile.id]:
                            profile.auth === 'key' ? { passphrase: value } : { password: value }
                        }))
                      }}
                    />
                  </label>
                )}
              {profile.auth !== 'agent' && (
                <p className="remote-hint">
                  {t(
                    snapshot.credentialStorageAvailable === false
                      ? 'Защищённое хранилище ОС недоступно. Пароль не сохранён.'
                      : profile.hasSavedCredentials
                        ? 'Пароль сохранён в защищённом хранилище ОС. Для замены введите новый.'
                        : 'Пароль будет сохранён в защищённом хранилище ОС и не передаётся в чат.'
                  )}
                </p>
              )}
              {profile.hasSavedCredentials && (
                <button
                  className="remote-forget-secret"
                  disabled={busy}
                  onClick={() => void request({ action: 'forgetCredentials', id: profile.id })}
                >
                  {t('Удалить сохранённый пароль')}
                </button>
              )}
              <div className="remote-card-actions">
                {profile.status === 'connected' ? (
                  <>
                    <button className="primary" onClick={() => onOpenProject(profile.id)}>
                      <ArrowUpRight size={15} />
                      {t('Открыть проект')}
                    </button>
                    <button
                      disabled={busy}
                      onClick={() => void request({ action: 'disconnect', id: profile.id })}
                    >
                      <Unplug size={15} />
                      {t('Отключить')}
                    </button>
                  </>
                ) : working ? (
                  <button
                    disabled={busy && profile.status === 'disconnecting'}
                    onClick={() =>
                      void api({ action: 'disconnect', id: profile.id })
                        .then(setSnapshot)
                        .catch((failure) => onError(remoteError(failure, t)))
                    }
                  >
                    <Unplug size={15} />
                    {t('Отменить')}
                  </button>
                ) : (
                  <>
                    <button
                      className="primary"
                      disabled={busy || readOnly || Boolean(profile.challenge)}
                      onClick={() => void connect(profile)}
                    >
                      <Cable size={15} />
                      {t('Подключить')}
                    </button>
                    <button
                      className="icon-button"
                      disabled={busy || readOnly}
                      aria-label={t('Изменить сервер')}
                      onClick={() => setEditor(profile)}
                    >
                      <Pencil size={15} />
                    </button>
                    <button
                      className="icon-button"
                      disabled={busy || readOnly}
                      aria-label={t('Удалить профиль')}
                      onClick={() => {
                        if (confirm === `remove:${profile.id}`) {
                          setConfirm('')
                          void request({ action: 'remove', id: profile.id })
                        } else setConfirm(`remove:${profile.id}`)
                      }}
                    >
                      <Trash2 size={15} />
                    </button>
                  </>
                )}
              </div>
              {confirm === `remove:${profile.id}` && (
                <p className="remote-hint">
                  {t('Нажмите удалить ещё раз. Файлы на сервере сохранятся.')}
                </p>
              )}
            </article>
          )
        })}
      </div>
      <p className="remote-footer">
        {t('Пароли не сохраняются. Локальные ключи API не передаются на сервер.')}
      </p>
    </section>
  )
}
