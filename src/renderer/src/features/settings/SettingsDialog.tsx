import { useI18n } from '../../i18n'
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
  const { t, language } = useI18n()
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
    { id: 'appearance', label: t('Интерфейс'), icon: Monitor },
    { id: 'providers', label: t('Провайдеры'), icon: Server },
    { id: 'advanced', label: t('Настройки Prime'), icon: SlidersHorizontal },
    { id: 'about', label: t('О приложении'), icon: Info }
  ]
  async function saveProvider() {
    setSaving(true)
    setStatus('')
    try {
      if (!/^[a-z0-9_-]+$/i.test(providerId))
        throw new Error(t('ID провайдера: латиница, цифры, _ и -'))
      const url = new URL(baseUrl)
      if (!['http:', 'https:'].includes(url.protocol))
        throw new Error(t('Нужен HTTP/HTTPS адрес API'))
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
      setStatus(t('Провайдер сохранён, агент готов к работе'))
      await w.refresh()
    } catch (error: any) {
      setStatus(error.message)
    } finally {
      setSaving(false)
    }
  }
  return (
    <Modal title={t('Настройки')} close={close} wide>
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
              <h3>{t('Ваше рабочее пространство')}</h3>
              <p className="muted">{t('Оформление приложения сохраняется на этом устройстве.')}</p>
              <label className="form-label">
                {t('Язык интерфейса')}
                <select
                  value={language}
                  onChange={(event) =>
                    void w.savePreferences({ language: event.target.value as 'ru' | 'en' })
                  }
                >
                  <option value="ru">{t('Русский')}</option>
                  <option value="en">{t('Английский')}</option>
                </select>
              </label>
              <label className="form-label">{t('Цветовая тема')}</label>
              <div className="theme-options">
                {(
                  [
                    { id: 'dark', label: t('Тёмная'), icon: Moon },
                    { id: 'light', label: t('Светлая'), icon: Sun },
                    { id: 'system', label: t('Системная'), icon: Monitor }
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
                <strong>{t('Клавиатура')}</strong>
                <p>
                  {t('Ctrl+N — новый чат · Ctrl+P — команды · Ctrl+L — модели')}
                  <br />
                  {t('Shift+Enter — новая строка · двойной Esc — остановить задачу')}
                  <br />
                  {t('Alt+V — голосовой ввод')}
                </p>
              </div>
            </>
          )}
          {tab === 'providers' && (
            <>
              <h3>{t('Подключение к модели')}</h3>
              <p className="muted">
                {t(
                  'Используются настройки и провайдеры Prime. Ключ хранится в конфигурации backend.'
                )}
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
                <select value={type} onChange={(event) => setType(event.target.value)}>
                  <option value="openai-compat">{t('Совместимый с OpenAI')}</option>
                  <option value="openai">OpenAI</option>
                  <option value="anthropic">Anthropic</option>
                  <option value="ollama">Ollama</option>
                </select>
              </label>
              <label className="form-label">
                {t('API ключ')}
                <input
                  type="password"
                  autoComplete="off"
                  value={key}
                  onChange={(event) => setKey(event.target.value)}
                  placeholder={t('Не требуется для локального сервера')}
                />
              </label>
              <div className="form-grid">
                <label className="form-label">
                  {t('ID модели')}
                  <input value={model} onChange={(event) => setModel(event.target.value)} />
                </label>
                <label className="form-label">
                  {t('Контекст')}
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
                {saving ? t('Подключаем…') : t('Сохранить и подключить')}
              </button>
              <h4>{t('Настроенные провайдеры')}</h4>
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
                    {t('Изменить')}
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
                      {t('Подтвердить')}
                    </button>
                  ) : (
                    <button className="quiet" onClick={() => setRemoving(id)}>
                      {t('Удалить')}
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
                      setStatus(t('Учётные данные Copilot импортированы'))
                      return w.refresh()
                    })
                    .catch((error) => setStatus(error.message))
                }
              >
                {t('Импортировать авторизацию Copilot')}
              </button>
            </>
          )}
          {tab === 'advanced' && (
            <>
              <h3>{t('Конфигурация Prime')}</h3>
              <p className="muted">
                {t('Изменения проходят через конфигурационный слой CLI. Значение задаётся в JSON.')}
              </p>
              <label className="form-label">
                {t('Область')}
                <select value={scope} onChange={(event) => setScope(Number(event.target.value))}>
                  <option value={0}>{t('Глобально')}</option>
                  <option value={1}>{t('Этот проект')}</option>
                </select>
              </label>
              <label className="form-label">
                {t('Ключ')}
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
                {t('Значение')}
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
                      setStatus(t('Сохранено'))
                    } catch (error: any) {
                      setStatus(error.message)
                    }
                  }}
                >
                  {t('Сохранить')}
                </button>
                <button
                  disabled={!w.project}
                  onClick={() =>
                    void w
                      .call('configRemove', { key: field, scope })
                      .then(w.refresh)
                      .then(() => setStatus(t('Значение удалено')))
                      .catch((error) => setStatus(error.message))
                  }
                >
                  {t('Сбросить поле')}
                </button>
              </div>
              <label className="form-label">
                {t('MCP-каналы этого проекта')}
                <input
                  value={channels}
                  onChange={(event) => setChannels(event.target.value)}
                  placeholder={t('Имена серверов через запятую')}
                />
              </label>
              <p className="muted">
                {t(
                  'Выбранные серверы смогут доставлять сообщения в рабочую область. Агент не запускается автоматически.'
                )}
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
                    t(
                      'Каналы сохранены. Закройте проект в боковой панели и откройте его снова, чтобы применить выбор.'
                    )
                  )
                }}
              >
                {t('Сохранить выбор каналов')}
              </button>
              <details>
                <summary>{t('Текущая конфигурация без ключей')}</summary>
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
              <p className="muted">{t('Локальное рабочее пространство для Prime.')}</p>
              <dl className="info-list">
                <dt>{t('Приложение')}</dt>
                <dd>{w.info.version || '0.1.0'}</dd>
                <dt>Prime CLI</dt>
                <dd>{w.info.runtimeVersion?.version || t('Запуск…')}</dd>
                <dt>{t('Среда выполнения')}</dt>
                <dd>
                  {(
                    {
                      ready: t('Готов'),
                      starting: t('Запуск…'),
                      error: t('Ошибка'),
                      stopped: t('Остановлен')
                    } as Record<string, string>
                  )[w.info.runtime] || w.info.runtime}
                </dd>
                <dt>{t('Соединение')}</dt>
                <dd>Windows named pipe</dd>
              </dl>
              <button
                onClick={async () => {
                  const result = await window.prime.update('check')
                  setStatus(
                    result.message ||
                      (result.state === 'current'
                        ? t('Установлена актуальная версия')
                        : t('Обновление: {value0}', { value0: result.version || result.state }))
                  )
                }}
              >
                <Download size={15} /> {t('Проверить обновления')}
              </button>
              <p className="muted small-text">
                {t(
                  'Локальная сборка. Публичный канал обновлений и цифровая подпись настраиваются при публикации.'
                )}
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
