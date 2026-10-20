import { useState } from 'react'
import { FolderKey, X } from 'lucide-react'
import type { RemoteProfileInput } from '../../../../shared/remote'
import { useI18n } from '../../i18n'
import { Select } from '../../components/Select'
type Props = {
  initial?: RemoteProfileInput
  busy: boolean
  onSave(value: RemoteProfileInput): Promise<boolean>
  onClose(): void
  chooseKey?(): Promise<string | undefined>
}
export function RemoteProfileEditor({ initial, busy, onSave, onClose, chooseKey }: Props) {
  const { t } = useI18n()
  const [value, setValue] = useState<RemoteProfileInput>(
    initial ?? {
      name: '',
      host: '',
      port: 22,
      user: '',
      root: '',
      auth: 'agent',
      runtime: 'bundled'
    }
  )
  const update = (patch: Partial<RemoteProfileInput>) =>
    setValue((current) => ({ ...current, ...patch }))
  const field = (
    key: 'name' | 'host' | 'user' | 'root' | 'keyPath' | 'runtimePath',
    label: string,
    placeholder?: string
  ) => (
    <label>
      {t(label)}
      <input
        required
        value={value[key] ?? ''}
        onChange={(event) => update({ [key]: event.target.value })}
        placeholder={placeholder}
        autoComplete="off"
      />
    </label>
  )
  return (
    <form
      className="remote-editor"
      onSubmit={async (event) => {
        event.preventDefault()
        if (await onSave(value)) onClose()
      }}
    >
      <div className="remote-editor-heading">
        <strong>{t(initial ? 'Изменить сервер' : 'Новый сервер')}</strong>
        <button type="button" className="icon-button" aria-label={t('Закрыть')} onClick={onClose}>
          <X size={16} />
        </button>
      </div>
      {field('name', 'Название', 'Production')}
      <div className="remote-fields">
        {field('host', 'Хост', 'server.example.com')}
        <label>
          {t('Порт')}
          <input
            required
            type="number"
            min="1"
            max="65535"
            value={value.port}
            onChange={(event) => update({ port: Number(event.target.value) })}
          />
        </label>
      </div>
      {field('user', 'Пользователь', 'developer')}
      {field('root', 'Папка проекта', '/home/developer/project')}
      <label>
        {t('Аутентификация')}
        <Select
          label={t('Аутентификация')}
          value={value.auth}
          onChange={(auth) => update({ auth: auth as RemoteProfileInput['auth'] })}
          options={[
            { value: 'agent', label: t('SSH-агент') },
            { value: 'key', label: t('Файл ключа') },
            { value: 'password', label: t('Пароль') }
          ]}
        />
      </label>
      {value.auth === 'key' && (
        <div className="remote-key-field">
          {field('keyPath', 'Приватный ключ')}
          {chooseKey && (
            <button
              type="button"
              aria-label={t('Выбрать ключ')}
              onClick={async () => {
                const keyPath = await chooseKey()
                if (keyPath) update({ keyPath })
              }}
            >
              <FolderKey size={17} />
            </button>
          )}
        </div>
      )}
      <p className="remote-hint">
        {t(
          'Модели и API-ключи остаются на этом компьютере. Команды и файлы проекта доступны через SSH.'
        )}
      </p>
      <button className="primary" disabled={busy} type="submit">
        {t('Сохранить')}
      </button>
    </form>
  )
}
