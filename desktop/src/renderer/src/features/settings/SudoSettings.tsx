import { useEffect, useState } from 'react'
import { useI18n } from '../../i18n'

const isLinux = navigator.platform.startsWith('Linux')

export function SudoSettings() {
  const { t } = useI18n()
  const [status, setStatus] = useState<{ hasPassword: boolean; storageAvailable: boolean }>()
  const [password, setPassword] = useState('')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  useEffect(() => {
    void window.prime
      .sudoPassword({ action: 'status' })
      .then(setStatus)
      .catch((error) => setMessage(error.message))
  }, [])
  async function save() {
    setSaving(true)
    setMessage('')
    try {
      setStatus(await window.prime.sudoPassword({ action: 'set', password }))
      setPassword('')
      setMessage(t('Сохранено'))
    } catch (error: any) {
      setMessage(error.message)
    } finally {
      setSaving(false)
    }
  }
  async function remove() {
    setSaving(true)
    setMessage('')
    try {
      setStatus(await window.prime.sudoPassword({ action: 'remove' }))
      setMessage(t('Пароль удалён'))
    } catch (error: any) {
      setMessage(error.message)
    } finally {
      setSaving(false)
    }
  }
  return (
    <>
      <h3>{t('Команды с root-правами (sudo)')}</h3>
      {isLinux ? (
        <>
          <p className="muted">
            {t(
              'Пароль root позволяет агенту выполнять системные команды через тул sudo. Он хранится в защищённом хранилище ОС и не виден модели.'
            )}
          </p>
          <fieldset className="settings-fields" disabled={saving}>
            <label className="form-label">
              {t('Пароль root (sudo)')}
              <input
                type="password"
                autoComplete="new-password"
                placeholder={
                  status?.hasPassword
                    ? t('Сохранённый пароль')
                    : status?.storageAvailable === false
                      ? t('Защищённое хранилище ОС недоступно')
                      : undefined
                }
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
            <div className="settings-actions">
              <button
                className="primary"
                disabled={saving || !password || status?.storageAvailable === false}
                onClick={() => void save()}
              >
                {t('Сохранить пароль')}
              </button>
              {status?.hasPassword && (
                <button disabled={saving} onClick={() => void remove()}>
                  {t('Удалить сохранённый пароль')}
                </button>
              )}
            </div>
          </fieldset>
        </>
      ) : (
        <p className="muted">{t('Доступно только на Linux.')}</p>
      )}
      {message && (
        <p
          role="status"
          className={`settings-status ${message === t('Сохранено') ? 'success' : ''}`}
        >
          {message}
        </p>
      )}
    </>
  )
}
