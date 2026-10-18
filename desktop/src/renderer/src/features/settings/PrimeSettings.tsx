import { useState } from 'react'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function PrimeSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [status, setStatus] = useState('')
  const [saving, setSaving] = useState(false)
  const [scope, setScope] = useState(0)
  const options = w.config.options || {}
  async function save(key: string, value: unknown) {
    setSaving(true)
    setStatus('')
    try {
      await w.call('configSet', { scope, key: `options.${key}`, value })
      await w.refresh()
      setStatus(t('Сохранено'))
    } catch (error: any) {
      setStatus(error.message)
    } finally {
      setSaving(false)
    }
  }
  return (
    <>
      <h3>{t('Настройки Prime')}</h3>
      <p className="muted">{t('Параметры агента сохраняются автоматически.')}</p>
      <label className="form-label">
        {t('Область')}
        <Select
          label={t('Область')}
          value={String(scope)}
          onChange={(value) => setScope(Number(value))}
          options={[
            { value: '0', label: t('Глобально') },
            ...(w.project?.kind === 'project' ? [{ value: '1', label: t('Этот проект') }] : [])
          ]}
        />
      </label>
      <fieldset className="settings-fields" disabled={saving || !w.project}>
        <label className="form-label">
          {t('Язык Prime')}
          <Select
            label={t('Язык Prime')}
            value={options.language || 'en'}
            onChange={(value) => void save('language', value)}
            options={[
              { value: 'ru', label: t('Русский') },
              { value: 'en', label: t('Английский') }
            ]}
          />
        </label>
        <label className="form-label">
          {t('Уведомления')}
          <Select
            label={t('Уведомления')}
            value={options.notifications || 'auto'}
            onChange={(value) => void save('notifications', value)}
            options={[
              { value: 'auto', label: t('Автоматически') },
              { value: 'native', label: t('Системные') },
              { value: 'disabled', label: t('Отключены') },
              { value: 'bell', label: t('Звуковой сигнал') },
              { value: 'osc', label: t('В терминале') }
            ]}
          />
        </label>
        {[
          {
            key: 'disable_auto_summarize',
            label: t('Автоматическое сжатие контекста'),
            value: !options.disable_auto_summarize,
            invert: true
          },
          {
            key: 'auto_lsp',
            label: t('Автоматический запуск LSP'),
            value: options.auto_lsp !== false
          },
          {
            key: 'auto_continue',
            label: t('Продолжать после ошибки провайдера'),
            value: !!options.auto_continue
          }
        ].map((item) => (
          <label className="setting-toggle" key={item.key}>
            <span>{item.label}</span>
            <input
              type="checkbox"
              checked={item.value}
              onChange={(event) =>
                void save(item.key, item.invert ? !event.target.checked : event.target.checked)
              }
            />
          </label>
        ))}
        <label className="form-label">
          {t('Сжимать контекст при заполнении, %')}
          <input
            key={options.auto_summarize_percent}
            type="number"
            min={1}
            max={100}
            defaultValue={options.auto_summarize_percent || 70}
            onBlur={(event) => {
              const value = Number(event.target.value)
              if (
                Number.isInteger(value) &&
                value >= 1 &&
                value <= 100 &&
                value !== (options.auto_summarize_percent || 70)
              )
                void save('auto_summarize_percent', value)
            }}
          />
        </label>
      </fieldset>
      {status && (
        <p role="status" className="settings-status">
          {status}
        </p>
      )}
    </>
  )
}
