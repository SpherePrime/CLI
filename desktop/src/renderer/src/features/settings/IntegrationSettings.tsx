import { useEffect, useRef, useState } from 'react'
import { Plus, RefreshCw, ArrowLeft, Trash2 } from 'lucide-react'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { IntegrationTools } from './IntegrationTools'

const segment = (value: string) => value.replace(/[\\.:*?#[\]]/g, '\\$&')

export function IntegrationSettings({
  workspace: w,
  kind
}: {
  workspace: Workspace
  kind: 'mcp' | 'lsp' | 'skills'
}) {
  const { t } = useI18n()
  const [editing, setEditing] = useState(false)
  const [original, setOriginal] = useState('')
  const [name, setName] = useState('')
  const [draft, setDraft] = useState<Record<string, any>>({})
  const [scope, setScope] = useState(0)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')
  const [removing, setRemoving] = useState('')
  const [skillContent, setSkillContent] = useState('')
  const [path, setPath] = useState('')
  const authTimer = useRef<ReturnType<typeof setInterval> | undefined>(undefined)
  useEffect(() => () => clearInterval(authTimer.current), [])
  const [secretRows, setSecretRows] = useState<{ group: string; key: string; value: string }[]>([])
  const configured = w.config[kind] || {}
  const runtime = w.resources[kind] || {}
  const entries =
    kind === 'skills'
      ? [
          ...(Array.isArray(runtime) ? runtime : []),
          ...(w.config.options?.disabled_skills || [])
            .filter(
              (name: string) =>
                !(Array.isArray(runtime) ? runtime : []).some((item: any) => item.name === name)
            )
            .map((name: string) => ({ name }))
        ]
      : Object.entries({ ...configured, ...runtime }).map(([name, value]: [string, any]) => ({
          ...value,
          name
        }))
  const paths: string[] = w.config.options?.skills_paths || []
  useEffect(() => {
    setSkillContent('')
  }, [kind])
  async function run(action: () => Promise<unknown>) {
    setSaving(true)
    setStatus('')
    try {
      await action()
      await w.refresh()
      setStatus(t('Сохранено'))
      return true
    } catch (error: any) {
      setStatus(error.message)
      return false
    } finally {
      setSaving(false)
    }
  }
  async function configure(args: any) {
    await w.call('configSet', args)
    if (kind === 'skills') await w.call('restart')
    if (kind === 'lsp') await w.call('lspStop')
  }
  async function authorize(name: string) {
    await run(async () => {
      const pending = w.call('mcpAuth', { name })
      let opened = false
      authTimer.current = setInterval(() => {
        void w
          .call('mcpAuthUrl', { name })
          .then(async (result) => {
            if (result.auth_url && !opened) {
              opened = true
              clearInterval(authTimer.current)
              await window.prime.openExternal(result.auth_url)
            }
          })
          .catch(() => clearInterval(authTimer.current))
      }, 600)
      try {
        await pending
      } finally {
        clearInterval(authTimer.current)
      }
    })
  }
  function edit(id = '') {
    setOriginal(id)
    setName(id)
    setDraft({
      ...(kind === 'mcp' ? { type: 'stdio' } : {}),
      timeout: kind === 'mcp' ? 10 : 30,
      ...configured[id]
    })
    setSecretRows([])
    setStatus('')
    setEditing(true)
  }
  function field(key: string, value: any) {
    setDraft((current) => ({ ...current, [key]: value }))
  }
  async function save() {
    const id = name.trim()
    if (!id || (!original && Object.hasOwn(configured, id))) {
      setStatus(t('Укажите уникальное название'))
      return
    }
    const stdio = kind === 'lsp' || draft.type === 'stdio'
    if (stdio && !draft.command?.trim()) {
      setStatus(t('Укажите команду запуска'))
      return
    }
    if (!stdio) {
      try {
        const url = new URL(draft.url)
        if (!['http:', 'https:'].includes(url.protocol)) throw new Error()
      } catch {
        setStatus(t('Укажите адрес HTTP или HTTPS'))
        return
      }
    }
    if (secretRows.some((row) => !row.key.trim() || !row.value)) {
      setStatus(t('Заполните название и значение'))
      return
    }
    if (!Number.isInteger(draft.timeout) || draft.timeout < 1 || draft.timeout > 600) {
      setStatus(t('Тайм-аут должен быть от 1 до 600 секунд'))
      return
    }
    const base = `${kind}.${segment(id)}`
    const values = { ...draft }
    if (values.args) values.args = values.args.filter((item: string) => item.trim())
    if (values.filetypes)
      values.filetypes = values.filetypes.map((item: string) => item.trim()).filter(Boolean)
    if (values.root_markers)
      values.root_markers = values.root_markers.map((item: string) => item.trim()).filter(Boolean)
    delete values.oauth_token
    delete values.oauth_client_secret
    delete values.env
    delete values.headers
    const ok = await run(async () => {
      if (!original) await configure({ scope, key: base, value: values })
      else
        for (const [key, value] of Object.entries(values)) {
          if (JSON.stringify(value) !== JSON.stringify(configured[id]?.[key]))
            await configure({ scope, key: `${base}.${key}`, value })
        }
      for (const row of secretRows)
        await configure({
          scope,
          key: `${base}.${row.group}.${segment(row.key.trim())}`,
          value: row.value
        })
    })
    if (ok) {
      setEditing(false)
      setSecretRows([])
    }
  }
  const disabledSkills: string[] = w.config.options?.disabled_skills || []
  return (
    <div className="integration-settings">
      <div className="integration-heading">
        <div>
          <h3>{kind === 'skills' ? 'Skills' : kind.toUpperCase()}</h3>
          <p className="muted">
            {kind === 'mcp'
              ? t('Подключения к инструментам и сервисам.')
              : kind === 'lsp'
                ? t('Языковые серверы для работы с кодом.')
                : t('Инструкции и навыки агента.')}
          </p>
        </div>
        <button
          className="quiet"
          disabled={saving}
          onClick={() => void run(() => w.refresh())}
          aria-label={t('Обновить')}
        >
          <RefreshCw size={16} />
        </button>
      </div>
      <label className="form-label">
        {t('Область')}
        <Select
          label={t('Область')}
          value={String(scope)}
          disabled={saving || editing}
          onChange={(value) => setScope(Number(value))}
          options={[
            { value: '0', label: t('Глобально') },
            ...(w.project?.kind === 'project' ? [{ value: '1', label: t('Этот проект') }] : [])
          ]}
        />
      </label>
      {!editing && (
        <button
          className="quiet"
          disabled={saving || w.busy || !w.project}
          onClick={() => void run(() => w.call('restart'))}
        >
          <RefreshCw size={14} />
          {t('Перезапустить системы')}
        </button>
      )}
      {kind !== 'skills' && !editing && (
        <button
          className="primary"
          disabled={saving || !w.project || w.busy}
          onClick={() => edit()}
        >
          <Plus size={16} />
          {t('Добавить сервер')}
        </button>
      )}
      {kind !== 'skills' && editing && (
        <form
          className="integration-editor"
          onSubmit={(event) => {
            event.preventDefault()
            void save()
          }}
        >
          <button
            type="button"
            className="quiet"
            disabled={saving}
            onClick={() => setEditing(false)}
          >
            <ArrowLeft size={15} />
            {t('К списку')}
          </button>
          <fieldset className="settings-fields" disabled={saving || w.busy}>
            <label className="form-label">
              {t('Название')}
              <input
                value={name}
                disabled={!!original}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </label>
            {kind === 'mcp' && (
              <label className="form-label">
                {t('Подключение')}
                <Select
                  label={t('Подключение')}
                  value={draft.type}
                  onChange={(value) => field('type', value)}
                  options={[
                    { value: 'stdio', label: t('Локальный процесс') },
                    { value: 'http', label: 'HTTP' },
                    { value: 'sse', label: 'SSE' }
                  ]}
                />
              </label>
            )}
            {kind === 'lsp' || draft.type === 'stdio' ? (
              <>
                <label className="form-label">
                  {t('Команда запуска')}
                  <input
                    value={draft.command || ''}
                    onChange={(event) => field('command', event.target.value)}
                    required
                    placeholder={kind === 'mcp' ? 'npx' : 'gopls'}
                  />
                </label>
                <label className="form-label">
                  {t('Аргументы — каждый с новой строки')}
                  <textarea
                    rows={3}
                    value={(draft.args || []).join('\n')}
                    onChange={(event) => field('args', event.target.value.split('\n'))}
                  />
                </label>
              </>
            ) : (
              <>
                <label className="form-label">
                  {t('Адрес сервера')}
                  <input
                    type="url"
                    value={draft.url || ''}
                    onChange={(event) => field('url', event.target.value)}
                    required
                  />
                </label>
                {draft.type === 'http' && (
                  <label className="setting-toggle">
                    <span>OAuth</span>
                    <input
                      type="checkbox"
                      checked={!!draft.oauth}
                      onChange={(event) => field('oauth', event.target.checked)}
                    />
                  </label>
                )}
              </>
            )}
            {kind === 'lsp' && (
              <>
                <label className="form-label">
                  {t('Расширения файлов — через запятую')}
                  <input
                    value={(draft.filetypes || []).join(',')}
                    onChange={(event) => field('filetypes', event.target.value.split(','))}
                  />
                </label>
                <label className="form-label">
                  {t('Файлы корня проекта — каждый с новой строки')}
                  <textarea
                    rows={2}
                    value={(draft.root_markers || []).join('\n')}
                    onChange={(event) => field('root_markers', event.target.value.split('\n'))}
                  />
                </label>
              </>
            )}
            <label className="form-label">
              {t('Тайм-аут, секунд')}
              <input
                type="number"
                min={1}
                max={600}
                value={draft.timeout ?? 30}
                onChange={(event) => field('timeout', Number(event.target.value))}
                required
              />
            </label>
            <label className="setting-toggle">
              <span>{t('Включено')}</span>
              <input
                type="checkbox"
                checked={!draft.disabled}
                onChange={(event) => field('disabled', !event.target.checked)}
              />
            </label>
            <details>
              <summary>{t('Переменные окружения и заголовки')}</summary>
              <p className="muted">
                {t('Сохранённые секреты скрыты и сохраняются при изменении других полей.')}
              </p>
              {secretRows.map((row, index) => (
                <div className="integration-secret" key={index}>
                  <Select
                    label={t('Тип параметра')}
                    value={row.group}
                    onChange={(value) =>
                      setSecretRows((items) =>
                        items.map((item, i) => (i === index ? { ...item, group: value } : item))
                      )
                    }
                    options={[
                      { value: 'env', label: t('Переменная окружения') },
                      ...(kind === 'mcp' ? [{ value: 'headers', label: t('HTTP-заголовок') }] : [])
                    ]}
                  />
                  <input
                    aria-label={t('Название параметра')}
                    placeholder={t('Название параметра')}
                    value={row.key}
                    onChange={(event) =>
                      setSecretRows((items) =>
                        items.map((item, i) =>
                          i === index ? { ...item, key: event.target.value } : item
                        )
                      )
                    }
                  />
                  <input
                    type="password"
                    aria-label={t('Значение параметра')}
                    placeholder={t('Значение параметра')}
                    value={row.value}
                    autoComplete="new-password"
                    onChange={(event) =>
                      setSecretRows((items) =>
                        items.map((item, i) =>
                          i === index ? { ...item, value: event.target.value } : item
                        )
                      )
                    }
                  />
                  <button
                    type="button"
                    className="icon"
                    aria-label={t('Убрать параметр')}
                    onClick={() => setSecretRows((items) => items.filter((_, i) => i !== index))}
                  >
                    <Trash2 size={15} />
                  </button>
                </div>
              ))}
              <button
                type="button"
                className="quiet"
                onClick={() =>
                  setSecretRows((items) => [...items, { group: 'env', key: '', value: '' }])
                }
              >
                <Plus size={14} />
                {t('Добавить параметр')}
              </button>
            </details>
            <button type="submit" className="primary">
              {saving ? t('Сохранение…') : t('Сохранить')}
            </button>
          </fieldset>
        </form>
      )}
      {!editing && (
        <div className="integration-list">
          {entries.map((item: any, index: number) => {
            const id = item.name || item.id || item.Name || String(index)
            const disabled =
              kind === 'skills'
                ? disabledSkills.includes(item.name || item.Name || item.id)
                : configured[id]?.disabled
            return (
              <section className="integration-card" key={id}>
                <div>
                  <strong>{id}</strong>
                  <p className="muted">
                    {item.description ||
                      configured[id]?.url ||
                      configured[id]?.command ||
                      item.path ||
                      item.source}
                  </p>
                  <small className={item.error ? 'error-text' : 'muted'}>
                    {disabled ? t('Отключено') : item.error ? String(item.error) : t('Доступно')}
                  </small>
                </div>
                <div className="button-row">
                  {kind !== 'skills' && (
                    <button disabled={saving || w.busy} onClick={() => edit(id)}>
                      {t('Изменить')}
                    </button>
                  )}
                  {kind === 'mcp' && configured[id]?.type === 'http' && (
                    <button
                      disabled={saving || w.busy || disabled}
                      onClick={() => void authorize(id)}
                    >
                      {t('Авторизация')}
                    </button>
                  )}
                  <button
                    disabled={saving || w.busy}
                    onClick={() =>
                      void run(() =>
                        configure({
                          scope,
                          key:
                            kind === 'skills'
                              ? 'options.disabled_skills'
                              : `${kind}.${segment(id)}.disabled`,
                          value:
                            kind === 'skills'
                              ? disabled
                                ? disabledSkills.filter((value) => value !== id)
                                : [...disabledSkills, id]
                              : !disabled
                        })
                      )
                    }
                  >
                    {disabled ? t('Включить') : t('Отключить')}
                  </button>
                  {kind === 'skills' && item.id && (
                    <button
                      disabled={saving}
                      onClick={() =>
                        void run(async () => {
                          const result = await w.call('readSkill', { id: item.id || id })
                          setSkillContent(
                            result.content
                              ? new TextDecoder().decode(
                                  Uint8Array.from(atob(result.content), (char) =>
                                    char.charCodeAt(0)
                                  )
                                )
                              : ''
                          )
                        })
                      }
                    >
                      {t('Открыть инструкции')}
                    </button>
                  )}
                  {kind !== 'skills' && configured[id] && (
                    <button
                      className="quiet danger"
                      disabled={saving || w.busy}
                      onClick={() => setRemoving(id)}
                    >
                      {t('Удалить')}
                    </button>
                  )}
                </div>
                {kind !== 'skills' && !disabled && (
                  <IntegrationTools workspace={w} kind={kind} name={id} />
                )}
                {removing === id && (
                  <div className="integration-confirm">
                    <span>{t('Удалить подключение?')}</span>
                    <button
                      disabled={saving}
                      onClick={() =>
                        void run(async () => {
                          await w.call('configRemove', { scope, key: `${kind}.${segment(id)}` })
                          if (kind === 'lsp') await w.call('lspStop')
                        }).then((ok) => {
                          if (ok) setRemoving('')
                        })
                      }
                    >
                      {t('Удалить')}
                    </button>
                    <button onClick={() => setRemoving('')}>{t('Отмена')}</button>
                  </div>
                )}
              </section>
            )
          })}
          {!entries.length && <p className="panel-help">{t('Не настроено')}</p>}
        </div>
      )}
      {kind === 'skills' && (
        <section className="integration-editor">
          <h4>{t('Папки с навыками')}</h4>
          <p className="muted">{t('Добавьте папку с каталогами, содержащими SKILL.md.')}</p>
          {paths.map((value) => (
            <div className="integration-path" key={value}>
              <span>{value}</span>
              <button
                className="icon"
                aria-label={t('Удалить папку')}
                disabled={saving || w.busy}
                onClick={() =>
                  void run(() =>
                    configure({
                      scope,
                      key: 'options.skills_paths',
                      value: paths.filter((item) => item !== value)
                    })
                  )
                }
              >
                <Trash2 size={15} />
              </button>
            </div>
          ))}
          <form
            onSubmit={(event) => {
              event.preventDefault()
              const value = path.trim()
              if (value && !paths.includes(value))
                void run(() =>
                  configure({ scope, key: 'options.skills_paths', value: [...paths, value] })
                ).then((ok) => {
                  if (ok) setPath('')
                })
            }}
          >
            <label className="form-label">
              {t('Путь к папке')}
              <input value={path} onChange={(event) => setPath(event.target.value)} required />
            </label>
            <button className="primary" disabled={saving || !path.trim() || w.busy}>
              <Plus size={15} />
              {t('Добавить папку')}
            </button>
          </form>
          {skillContent && <pre className="skill-instructions">{skillContent}</pre>}
        </section>
      )}
      {kind === 'mcp' && !editing && (
        <section className="integration-editor">
          <h4>{t('MCP-каналы этого проекта')}</h4>
          <p className="muted">
            {t(
              'Выбранные серверы смогут доставлять сообщения в рабочую область. Агент не запускается автоматически.'
            )}
          </p>
          {Object.keys(configured).map((name) => (
            <label className="setting-toggle" key={name}>
              <span>{name}</span>
              <input
                type="checkbox"
                disabled={saving || !w.project}
                checked={!!w.preferences.projectChannels?.[w.project?.path || '']?.includes(name)}
                onChange={(event) => {
                  const selected = w.preferences.projectChannels?.[w.project?.path || ''] || []
                  void run(() =>
                    w.savePreferences({
                      projectChannels: {
                        ...w.preferences.projectChannels,
                        [w.project!.path]: event.target.checked
                          ? [...selected, name]
                          : selected.filter((item) => item !== name)
                      }
                    })
                  )
                }}
              />
            </label>
          ))}
          <p className="muted">
            {t(
              'Каналы сохранены. Закройте проект в боковой панели и откройте его снова, чтобы применить выбор.'
            )}
          </p>
          <details>
            <summary>{t('Docker MCP')}</summary>
            <div className="button-row">
              <button
                disabled={saving || w.busy}
                onClick={() => void run(() => w.call('dockerEnable'))}
              >
                {t('Включить')}
              </button>
              <button
                disabled={saving || w.busy}
                onClick={() => void run(() => w.call('dockerDisable'))}
              >
                {t('Отключить')}
              </button>
            </div>
          </details>
          {(w.resources.channels || []).map((message: any) => (
            <details key={message.id} className="job">
              <summary>
                {message.name} · {new Date(message.received_at).toLocaleTimeString()}
              </summary>
              <pre className="skill-instructions">{message.content}</pre>
              <button
                onClick={() =>
                  window.dispatchEvent(new CustomEvent('prime:draft', { detail: message.content }))
                }
              >
                {t('Подготовить запрос в чате')}
              </button>
            </details>
          ))}
        </section>
      )}
      {status && (
        <p role="status" className="settings-status">
          {status}
        </p>
      )}
    </div>
  )
}
