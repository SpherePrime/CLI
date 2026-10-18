import { useState } from 'react'
import { Plus, Trash2, Server } from 'lucide-react'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ProviderSettings({ workspace: w, scope }: { workspace: Workspace; scope: number }) {
  const { t } = useI18n()
  const [providerId, setProviderId] = useState('')
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [type, setType] = useState('openai-compat')
  const [key, setKey] = useState('')
  const [models, setModels] = useState<any[]>([])
  const [selected, setSelected] = useState(0)
  const [discover, setDiscover] = useState(true)
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')
  const [removing, setRemoving] = useState('')
  const [originalId, setOriginalId] = useState('')
  const [preset, setPreset] = useState('custom')
  function load(id: string, provider: any) {
    setProviderId(id)
    setOriginalId(w.config.providers?.[id] ? id : '')
    setName(provider.name || id)
    setBaseUrl(provider.base_url || '')
    setType(provider.type || 'openai-compat')
    setKey('')
    setModels(structuredClone(provider.models || []))
    setSelected(0)
    setDiscover(provider.discover_models ?? true)
    setEditing(true)
    setStatus('')
    setPreset('custom')
  }
  function applyPreset(value: string) {
    const entries: Record<string, any> = {
      deepseek: {
        name: 'DeepSeek',
        base_url: 'https://api.deepseek.com/v1',
        models: [
          {
            id: 'deepseek-chat',
            name: 'DeepSeek Chat',
            context_window: 64000,
            default_max_tokens: 8192
          }
        ]
      },
      openai: { name: 'OpenAI', base_url: 'https://api.openai.com/v1' },
      ollama: { name: 'Ollama', base_url: 'http://localhost:11434/v1' },
      custom: { name: '', base_url: '' }
    }
    if (w.config.providers?.[value]) {
      load(value, w.config.providers[value])
      setPreset(value)
      return
    }
    load(value === 'custom' ? '' : value, {
      ...entries[value],
      type: value === 'ollama' ? 'ollama' : 'openai-compat'
    })
    setPreset(value)
  }
  function patchModel(field: string, value: any) {
    setModels((items) =>
      items.map((item, index) => (index === selected ? { ...item, [field]: value } : item))
    )
  }
  async function save() {
    setStatus('')
    setSaving(true)
    try {
      if (!/^[a-z0-9_-]+$/i.test(providerId))
        throw new Error(t('ID провайдера: латиница, цифры, _ и -'))
      if (!['http:', 'https:'].includes(new URL(baseUrl).protocol))
        throw new Error(t('Нужен HTTP/HTTPS адрес API'))
      if (
        models.some((item) => !item.id?.trim()) ||
        new Set(models.map((item) => item.id)).size !== models.length
      )
        throw new Error(t('У каждой модели должен быть уникальный ID'))
      if (
        models.some(
          (item) =>
            !Number.isInteger(item.context_window) ||
            item.context_window < 1024 ||
            !Number.isInteger(item.default_max_tokens) ||
            item.default_max_tokens < 1 ||
            item.default_max_tokens > item.context_window
        )
      )
        throw new Error(t('Проверьте контекст и лимит ответа у каждой модели'))
      if (!discover && !models.length)
        throw new Error(t('Добавьте хотя бы одну модель или включите автопоиск моделей'))
      for (const [field, value] of Object.entries({
        ...(key ? { api_key: key } : {}),
        id: providerId,
        name: name || providerId,
        base_url: baseUrl,
        type,
        discover_models: discover,
        models
      }))
        await w.call('configSet', { key: `providers.${providerId}.${field}`, value, scope })
      setKey('')
      await w.refresh()
      const loaded = await w.call('models').catch(() => [])
      const list = Array.isArray(loaded) ? loaded : loaded?.models || []
      if (!list.some((item: any) => item.provider_id === providerId))
        throw new Error(
          models.length
            ? t('Провайдер сохранён, но Prime не смог его загрузить. Проверьте адрес API и тип.')
            : t(
                'Провайдер сохранён, но модели не найдены: сервер не отдал список моделей по этому адресу. Добавьте модели вручную или проверьте адрес API.'
              )
        )
      setStatus(t('Настройки провайдера сохранены'))
    } catch (error: any) {
      setStatus(error.message)
    } finally {
      setSaving(false)
    }
  }
  const model = models[selected]
  return (
    <>
      <div className="provider-title">
        <div>
          <h3>{t('Провайдеры и модели')}</h3>
          <p className="muted">{t('Ваши подключения Prime доступны во всех чатах.')}</p>
        </div>
        <button className="quiet" onClick={() => applyPreset('custom')}>
          <Plus size={15} />
          {t('Добавить провайдера')}
        </button>
      </div>
      <div className="provider-cards">
        {Object.entries(w.config.providers || {}).map(([id, provider]: [string, any]) => (
          <div
            className={`provider-card ${editing && providerId === id ? 'selected' : ''}`}
            key={id}
          >
            <Server size={19} />
            <div>
              <strong>{provider.name || id}</strong>
              <small>{provider.base_url}</small>
              <small>{t('Моделей: {count}', { count: provider.models?.length || 0 })}</small>
            </div>
            <button className="quiet" onClick={() => load(id, provider)}>
              {t('Изменить')}
            </button>
            <button
              className={removing === id ? 'danger' : 'quiet'}
              onClick={() => {
                if (removing !== id) {
                  setRemoving(id)
                  return
                }
                void w
                  .call('configRemove', { key: `providers.${id}`, scope })
                  .then(w.refresh)
                  .then(() => {
                    setRemoving('')
                    if (providerId === id) setEditing(false)
                  })
                  .catch(w.errorHandler)
              }}
            >
              {removing === id ? t('Подтвердить') : t('Удалить')}
            </button>
          </div>
        ))}
      </div>
      {!Object.keys(w.config.providers || {}).length && !editing && (
        <p className="muted">{t('Добавьте подключение к сервису или локальному серверу.')}</p>
      )}
      {editing && (
        <div className="provider-editor">
          <h4>{t('Настройка подключения')}</h4>
          <label className="form-label">
            {t('Тип подключения')}
            <Select
              label={t('Тип подключения')}
              value={preset}
              onChange={applyPreset}
              options={[
                { value: 'custom', label: t('Свой сервер / другой провайдер') },
                { value: 'deepseek', label: 'DeepSeek' },
                { value: 'openai', label: 'OpenAI' },
                { value: 'ollama', label: t('Ollama · локальные модели') }
              ]}
            />
          </label>
          <div className="form-grid">
            <label className="form-label">
              ID
              <input
                aria-label={t('ID провайдера')}
                disabled={Boolean(originalId)}
                value={providerId}
                onChange={(event) => setProviderId(event.target.value)}
              />
            </label>
            <label className="form-label">
              {t('Название')}
              <input value={name} onChange={(event) => setName(event.target.value)} />
            </label>
          </div>
          <label className="form-label">
            {t('Адрес API')}
            <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} />
          </label>
          <label className="form-label">
            {t('Формат API')}
            <Select
              label={t('Формат API')}
              value={type}
              onChange={setType}
              options={[
                { value: 'openai-compat', label: t('Совместимый с OpenAI') },
                { value: 'openai', label: 'OpenAI' },
                { value: 'anthropic', label: 'Anthropic' },
                { value: 'ollama', label: 'Ollama' }
              ]}
            />
          </label>
          <label className="form-label">
            {t('API ключ')}
            <input
              type="password"
              autoComplete="off"
              value={key}
              onChange={(event) => setKey(event.target.value)}
              placeholder={t('Оставьте пустым, чтобы сохранить текущий ключ')}
            />
          </label>
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={discover}
              onChange={(event) => setDiscover(event.target.checked)}
            />
            {t('Получать каталог моделей от провайдера')}
          </label>
          <div className="provider-model-heading">
            <h4>{t('Модели провайдера')}</h4>
            <button
              className="quiet"
              onClick={() => {
                setModels((items) => [
                  ...items,
                  { id: '', name: '', context_window: 64000, default_max_tokens: 8192 }
                ])
                setSelected(models.length)
              }}
            >
              <Plus size={14} />
              {t('Добавить модель')}
            </button>
          </div>
          {models.length > 0 && (
            <div className="provider-model-editor">
              <div
                className="provider-model-tabs"
                role="tablist"
                aria-label={t('Модели провайдера')}
              >
                {models.map((item, index) => (
                  <button
                    key={index}
                    role="tab"
                    aria-selected={selected === index}
                    onClick={() => setSelected(index)}
                  >
                    {item.name || item.id || t('Новая модель')}
                    <small>{item.id}</small>
                  </button>
                ))}
              </div>
              {model && (
                <div className="provider-model-fields" role="tabpanel">
                  <label className="form-label">
                    {t('ID модели')}
                    <input
                      value={model.id}
                      onChange={(event) => patchModel('id', event.target.value)}
                    />
                  </label>
                  <label className="form-label">
                    {t('Название модели')}
                    <input
                      value={model.name || ''}
                      onChange={(event) => patchModel('name', event.target.value)}
                    />
                  </label>
                  <div className="form-grid">
                    <label className="form-label">
                      {t('Контекст')}
                      <input
                        type="number"
                        inputMode="numeric"
                        min={1024}
                        value={model.context_window ?? ''}
                        onChange={(event) =>
                          patchModel(
                            'context_window',
                            event.target.value === '' ? '' : Number(event.target.value)
                          )
                        }
                      />
                    </label>
                    <label className="form-label">
                      {t('Лимит ответа')}
                      <input
                        type="number"
                        inputMode="numeric"
                        min={1}
                        value={model.default_max_tokens ?? ''}
                        onChange={(event) =>
                          patchModel(
                            'default_max_tokens',
                            event.target.value === '' ? '' : Number(event.target.value)
                          )
                        }
                      />
                    </label>
                  </div>
                  <button
                    className="quiet danger-text"
                    onClick={() => {
                      setModels((items) => items.filter((_, index) => index !== selected))
                      setSelected(Math.max(0, selected - 1))
                    }}
                  >
                    <Trash2 size={14} />
                    {t('Удалить модель')}
                  </button>
                </div>
              )}
            </div>
          )}
          <button className="primary" disabled={saving || !w.project} onClick={() => void save()}>
            {saving ? t('Сохраняем…') : t('Сохранить изменения')}
          </button>
        </div>
      )}
      {status && <p role="status">{status}</p>}
      <button
        className="quiet"
        disabled={!w.project}
        onClick={() =>
          void w
            .call('importCopilot')
            .then(w.refresh)
            .then(() => setStatus(t('Учётные данные Copilot импортированы')))
            .catch(w.errorHandler)
        }
      >
        {t('Импортировать авторизацию Copilot')}
      </button>
    </>
  )
}
