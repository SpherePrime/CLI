import { useState } from 'react'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ImageGenSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [status, setStatus] = useState('')
  const [saving, setSaving] = useState(false)
  const [scope, setScope] = useState(0)
  const imageGen = w.config.options?.image_gen || {}
  async function save(field: string, value: unknown) {
    setSaving(true)
    setStatus('')
    try {
      await w.call('configSet', { scope, key: `options.image_gen.${field}`, value })
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
      <h3>{t('Генерация изображений')}</h3>
      <p className="muted">
        {t('Инструмент generate_image работает через любой OpenAI-совместимый API изображений.')}
      </p>
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
          {t('API-ключ')}
          <input
            type="password"
            key={imageGen.api_key || ''}
            defaultValue={imageGen.api_key || ''}
            placeholder="sk-..."
            onBlur={(event) => {
              if (event.target.value !== (imageGen.api_key || ''))
                void save('api_key', event.target.value)
            }}
          />
        </label>
        <label className="form-label">
          {t('Модель изображений')}
          <input
            type="text"
            key={imageGen.model || ''}
            defaultValue={imageGen.model || 'gpt-image-1'}
            onBlur={(event) => {
              if (event.target.value !== (imageGen.model || 'gpt-image-1'))
                void save('model', event.target.value)
            }}
          />
        </label>
        <label className="form-label">
          {t('Base URL')}
          <input
            type="text"
            key={imageGen.base_url || ''}
            defaultValue={imageGen.base_url || 'https://api.openai.com/v1'}
            placeholder="https://api.openai.com/v1"
            onBlur={(event) => {
              if (event.target.value !== (imageGen.base_url || ''))
                void save('base_url', event.target.value)
            }}
          />
        </label>
        <label className="form-label">
          {t('Папка сохранения')}
          <input
            type="text"
            key={imageGen.output_dir || ''}
            defaultValue={imageGen.output_dir || ''}
            placeholder="./generated-images"
            onBlur={(event) => {
              if (event.target.value !== (imageGen.output_dir || ''))
                void save('output_dir', event.target.value)
            }}
          />
        </label>
        <label className="form-label">
          {t('Размер по умолчанию')}
          <input
            type="text"
            key={imageGen.size || ''}
            defaultValue={imageGen.size || ''}
            placeholder="1024x1024"
            onBlur={(event) => {
              if (event.target.value !== (imageGen.size || ''))
                void save('size', event.target.value)
            }}
          />
        </label>
        <label className="form-label">
          {t('Формат ответа')}
          <Select
            label={t('Формат ответа')}
            value={imageGen.response_format || ''}
            onChange={(value) => void save('response_format', value)}
            options={[
              { value: '', label: t('Автоматически') },
              { value: 'b64_json', label: 'b64_json' },
              { value: 'url', label: 'url' }
            ]}
          />
        </label>
        <label className="form-label">
          {t('Таймаут, мс')}
          <input
            type="number"
            min={1000}
            key={imageGen.timeout_ms ?? ''}
            defaultValue={imageGen.timeout_ms || 180000}
            onBlur={(event) => {
              const value = Number(event.target.value)
              if (Number.isInteger(value) && value >= 1000 && value !== (imageGen.timeout_ms || 180000))
                void save('timeout_ms', value)
            }}
          />
        </label>
      </fieldset>
      {status && (
        <p
          role="status"
          className={`settings-status ${status === t('Сохранено') ? 'success' : ''}`}
        >
          {status}
        </p>
      )}
    </>
  )
}
