import { useState, type RefObject } from 'react'
import type { LaunchConfig, LaunchConfigInput } from '../../../../shared/launch'
import { useI18n } from '../../i18n'

export function LaunchEditor({
  config,
  busy,
  inputRef,
  onSave,
  onCancel
}: {
  config?: LaunchConfig
  busy: boolean
  inputRef?: RefObject<HTMLInputElement | null>
  onSave(input: LaunchConfigInput): void
  onCancel(): void
}) {
  const { t } = useI18n()
  const [name, setName] = useState(config?.name ?? '')
  const [command, setCommand] = useState(config?.command ?? '')
  const [url, setUrl] = useState(config?.url ?? '')
  return (
    <form
      className="automation-editor launch-editor"
      onSubmit={(event) => {
        event.preventDefault()
        onSave({ id: config?.id, name, command, ...(url.trim() ? { url } : {}) })
      }}
    >
      <p className="launch-editor-description">
        {t(
          'Сохраните команду запуска для этого проекта. Prime покажет её вывод и адрес приложения.'
        )}
      </p>
      <label>
        {t('Название')}
        <input
          ref={inputRef}
          required
          maxLength={80}
          placeholder={t('Например, Dev server')}
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </label>
      <label>
        {t('Команда')}
        <input
          required
          maxLength={4000}
          placeholder="npm run dev"
          value={command}
          onChange={(event) => setCommand(event.target.value)}
        />
      </label>
      <label>
        {t('Адрес (необязательно)')}
        <input
          maxLength={2000}
          placeholder="http://localhost:5173"
          value={url}
          onChange={(event) => setUrl(event.target.value)}
        />
        <small>{t('Оставьте пустым, чтобы определить адрес из вывода команды.')}</small>
      </label>
      <div className="automation-actions">
        <button type="button" onClick={onCancel}>
          {t('Отмена')}
        </button>
        <button className="automation-primary" type="submit" disabled={busy}>
          {t('Сохранить')}
        </button>
      </div>
    </form>
  )
}
