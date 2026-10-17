import { PrimeLogo } from '../../components/PrimeLogo'
import { useState } from 'react'
import {
  Check,
  KeyRound,
  Monitor,
  Server,
  SlidersHorizontal,
  Info,
  Sun,
  Moon,
  Download
} from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function SettingsDialog({
  workspace: w,
  close,
  initial = 'appearance'
}: {
  workspace: Workspace
  close: () => void
  initial?: string
}) {
  const [tab, setTab] = useState(initial)
  const [providerId, setProviderId] = useState('custom')
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('https://api.deepseek.com/v1')
  const [type, setType] = useState('openai-compat')
  const [key, setKey] = useState('')
  const [model, setModel] = useState('deepseek-chat')
  const [contextWindow, setContextWindow] = useState(64000)
  const [status, setStatus] = useState('')
  const [saving, setSaving] = useState(false)
  const [field, setField] = useState('options.language')
  const [value, setValue] = useState('"ru"')
  const [scope, setScope] = useState(0)
  const [removing, setRemoving] = useState('')
  const [channels, setChannels] = useState(
    w.preferences.projectChannels?.[w.project?.path || '']?.join(', ') || ''
  )
  const tabs = [
    { id: 'appearance', label: 'Интерфейс', icon: Monitor },
    { id: 'providers', label: 'Провайдеры', icon: Server },
    { id: 'advanced', label: 'Настройки Prime', icon: SlidersHorizontal },
    { id: 'about', label: 'О приложении', icon: Info }
  ]
  async function saveProvider() {
    setSaving(true)
    setStatus('')
    try {
      if (!/^[a-z0-9_-]+$/i.test(providerId))
        throw new Error('ID провайдера: латиница, цифры, _ и -')
      const url = new URL(baseUrl)
      if (!['http:', 'https:'].includes(url.protocol)) throw new Error('Нужен HTTP/HTTPS адрес API')
      const existing = w.config.providers?.[providerId]
      const previousModels: any[] = existing?.models || []
      const editedModel = model
        ? {
            ...previousModels.find((item) => item.id === model),
            id: model,
            name: model,
            context_window: contextWindow,
            default_max_tokens:
              previousModels.find((item) => item.id === model)?.default_max_tokens || 8192
          }
        : null
      const entry = {
        id: providerId,
        name: name || providerId,
        base_url: baseUrl,
        type,
        discover_models: true,
        models: editedModel
          ? [...previousModels.filter((item) => item.id !== model), editedModel]
          : previousModels
      }
      for (const [field, value] of Object.entries(entry))
        await w.call('configSet', { key: `providers.${providerId}.${field}`, value, scope })
      if (key) await w.call('providerKey', { providerId, key, scope })
      if (model) await w.call('model', { scope, model: { provider: providerId, model } })
      await w.call('init')
      setKey('')
      setStatus('Провайдер сохранён, агент готов к работе')
      await w.refresh()
    } catch (error: any) {
      setStatus(error.message)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Modal title="Настройки" close={close} wide>
      <div className="settings-layout">
        <nav>
          {tabs.map((item) => (
            <button
              key={item.id}
              className={tab === item.id ? 'active' : ''}
              onClick={() => {
                setTab(item.id)
                setStatus('')
              }}
            >
              <item.icon size={16} />
              {item.label}
            </button>
          ))}
        </nav>
        <div className="settings-body">
          {tab === 'appearance' && (
            <>
              <h3>Ваше рабочее пространство</h3>
              <p className="muted">Оформление приложения сохраняется на этом устройстве.</p>
              <label className="form-label">Цветовая тема</label>
              <div className="theme-options">
                {(
                  [
                    { id: 'dark', label: 'Тёмная', icon: Moon },
                    { id: 'light', label: 'Светлая', icon: Sun },
                    { id: 'system', label: 'Системная', icon: Monitor }
                  ] as const
                ).map((theme) => (
                  <button
                    key={theme.id}
                    className={w.preferences.theme === theme.id ? 'selected' : ''}
                    onClick={() => void w.savePreferences({ theme: theme.id })}
                  >
                    <theme.icon size={22} />
                    {theme.label}
                    {w.preferences.theme === theme.id && <Check size={13} />}
                  </button>
                ))}
              </div>
              <div className="settings-note">
                <strong>Клавиатура</strong>
                <p>
                  Ctrl+N — новый чат · Ctrl+P — команды · Ctrl+L — модели
                  <br />
                  Shift+Enter — новая строка · двойной Esc — остановить задачу
                  <br />
                  Alt+V — голосовой ввод
                </p>
              </div>
            </>
          )}
          {tab === 'providers' && (
            <>
              <h3>Подключение к модели</h3>
              <p className="muted">
                Используются настройки и провайдеры Prime. Ключ хранится в конфигурации backend.
              </p>
              <div className="provider-presets">
                {[
                  {
                    id: 'deepseek',
                    name: 'DeepSeek',
                    url: 'https://api.deepseek.com/v1',
                    model: 'deepseek-chat'
                  },
                  {
                    id: 'openai',
                    name: 'OpenAI',
                    url: 'https://api.openai.com/v1',
                    model: 'gpt-4.1'
                  },
                  { id: 'ollama', name: 'Ollama', url: 'http://localhost:11434/v1', model: '' }
                ].map((preset) => (
                  <button
                    key={preset.id}
                    onClick={() => {
                      setProviderId(preset.id)
                      setName(preset.name)
                      setBaseUrl(preset.url)
                      setModel(preset.model)
                      setType(preset.id === 'ollama' ? 'ollama' : 'openai-compat')
                    }}
                  >
                    {preset.name}
                  </button>
                ))}
              </div>
              <div className="form-grid">
                <label className="form-label">
                  ID
                  <input
                    value={providerId}
                    onChange={(event) => setProviderId(event.target.value)}
                  />
                </label>
                <label className="form-label">
                  Название
                  <input value={name} onChange={(event) => setName(event.target.value)} />
                </label>
              </div>
              <label className="form-label">
                Адрес API
                <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} />
              </label>
              <label className="form-label">
                Формат API
                <select value={type} onChange={(event) => setType(event.target.value)}>
                  <option value="openai-compat">OpenAI compatible</option>
                  <option value="openai">OpenAI</option>
                  <option value="anthropic">Anthropic</option>
                  <option value="ollama">Ollama</option>
                </select>
              </label>
              <label className="form-label">
                API ключ
                <input
                  type="password"
                  autoComplete="off"
                  value={key}
                  onChange={(event) => setKey(event.target.value)}
                  placeholder="Не требуется для локального сервера"
                />
              </label>
              <div className="form-grid">
                <label className="form-label">
                  ID модели
                  <input value={model} onChange={(event) => setModel(event.target.value)} />
                </label>
                <label className="form-label">
                  Контекст
                  <input
                    type="number"
                    min={1024}
                    value={contextWindow}
                    onChange={(event) => setContextWindow(Number(event.target.value))}
                  />
                </label>
              </div>
              <button
                className="primary"
                disabled={saving || !w.project}
                onClick={() => void saveProvider()}
              >
                {saving ? 'Подключаем…' : 'Сохранить и подключить'}
              </button>
              <h4>Настроенные провайдеры</h4>
              {Object.entries(w.config.providers || {}).map(([id, provider]: [string, any]) => (
                <div key={id} className="configured-provider">
                  <span>
                    {provider.name || id}
                    <small>{provider.base_url}</small>
                  </span>
                  <button
                    className="quiet"
                    onClick={() => {
                      setProviderId(id)
                      setName(provider.name || id)
                      setBaseUrl(provider.base_url || '')
                      setType(provider.type || 'openai-compat')
                      setModel(provider.models?.[0]?.id || '')
                    }}
                  >
                    Изменить
                  </button>
                  {removing === id ? (
                    <button
                      className="danger"
                      onClick={() =>
                        void w
                          .call('configRemove', { key: `providers.${id}`, scope })
                          .then(w.refresh)
                          .then(() => setRemoving(''))
                          .catch(w.errorHandler)
                      }
                    >
                      Подтвердить
                    </button>
                  ) : (
                    <button className="quiet" onClick={() => setRemoving(id)}>
                      Удалить
                    </button>
                  )}
                </div>
              ))}
              <button
                className="quiet"
                disabled={!w.project}
                onClick={() =>
                  void w
                    .call('importCopilot')
                    .then(() => {
                      setStatus('Учётные данные Copilot импортированы')
                      return w.refresh()
                    })
                    .catch((error) => setStatus(error.message))
                }
              >
                Импортировать авторизацию Copilot
              </button>
            </>
          )}
          {tab === 'advanced' && (
            <>
              <h3>Конфигурация Prime</h3>
              <p className="muted">
                Изменения проходят через конфигурационный слой CLI. Значение задаётся в JSON.
              </p>
              <label className="form-label">
                Область
                <select value={scope} onChange={(event) => setScope(Number(event.target.value))}>
                  <option value={0}>Глобально</option>
                  <option value={1}>Этот проект</option>
                </select>
              </label>
              <label className="form-label">
                Ключ
                <input
                  value={field}
                  onChange={(event) => setField(event.target.value)}
                  list="config-keys"
                />
                <datalist id="config-keys">
                  {[
                    'options.language',
                    'options.notifications',
                    'options.auto_summarize',
                    'permissions.allowed_tools',
                    'mcp',
                    'lsp',
                    'skills'
                  ].map((item) => (
                    <option key={item}>{item}</option>
                  ))}
                </datalist>
              </label>
              <label className="form-label">
                Значение
                <textarea
                  rows={6}
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                />
              </label>
              <div className="button-row">
                <button
                  className="primary"
                  disabled={!w.project}
                  onClick={async () => {
                    try {
                      await w.call('configSet', { key: field, value: JSON.parse(value), scope })
                      await w.refresh()
                      setStatus('Сохранено')
                    } catch (error: any) {
                      setStatus(error.message)
                    }
                  }}
                >
                  Сохранить
                </button>
                <button
                  disabled={!w.project}
                  onClick={() =>
                    void w
                      .call('configRemove', { key: field, scope })
                      .then(w.refresh)
                      .then(() => setStatus('Значение удалено'))
                      .catch((error) => setStatus(error.message))
                  }
                >
                  Сбросить поле
                </button>
              </div>
              <label className="form-label">
                MCP-каналы этого проекта
                <input
                  value={channels}
                  onChange={(event) => setChannels(event.target.value)}
                  placeholder="Имена серверов через запятую"
                />
              </label>
              <p className="muted">
                Выбранные серверы смогут доставлять сообщения в рабочую область. Агент не
                запускается автоматически.
              </p>
              <button
                disabled={!w.project}
                onClick={async () => {
                  if (!w.project) return
                  await w.savePreferences({
                    projectChannels: {
                      ...w.preferences.projectChannels,
                      [w.project.path]: channels
                        .split(',')
                        .map((value) => value.trim())
                        .filter(Boolean)
                    }
                  })
                  setStatus(
                    'Каналы сохранены. Закройте проект в боковой панели и откройте его снова, чтобы применить выбор.'
                  )
                }}
              >
                Сохранить выбор каналов
              </button>
              <details>
                <summary>Текущая конфигурация без ключей</summary>
                <pre className="config-preview">{JSON.stringify(w.config, null, 2)}</pre>
              </details>
            </>
          )}
          {tab === 'about' && (
            <>
              <div className="about-symbol">
                <PrimeLogo />
              </div>
              <h3>Prime Desktop</h3>
              <p className="muted">Локальное рабочее пространство для Prime.</p>
              <dl className="info-list">
                <dt>Приложение</dt>
                <dd>{w.info.version || '0.1.0'}</dd>
                <dt>Prime CLI</dt>
                <dd>{w.info.runtimeVersion?.version || 'Запуск…'}</dd>
                <dt>Runtime</dt>
                <dd>{w.info.runtime}</dd>
                <dt>Соединение</dt>
                <dd>Windows named pipe</dd>
              </dl>
              <button
                onClick={async () => {
                  const result = await window.prime.update('check')
                  setStatus(
                    result.message ||
                      (result.state === 'current'
                        ? 'Установлена актуальная версия'
                        : `Обновление: ${result.version || result.state}`)
                  )
                }}
              >
                <Download size={15} /> Проверить обновления
              </button>
              <p className="muted small-text">
                Локальная сборка. Публичный канал обновлений и цифровая подпись настраиваются при
                публикации.
              </p>
              {w.info.error && <pre>{w.info.error}</pre>}
            </>
          )}
          {status && (
            <p className="settings-status" role="status">
              {status}
            </p>
          )}
        </div>
      </div>
    </Modal>
  )
}
