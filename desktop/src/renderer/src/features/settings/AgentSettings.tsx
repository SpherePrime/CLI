import { useEffect, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'

export function AgentSettings({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const options = w.config.options || {}
  const limit = options.max_concurrent_agents || 10
  const [draft, setDraft] = useState(String(limit))
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    setDraft(String(limit))
  }, [limit])
  async function save(key: string, value: unknown) {
    setSaving(true)
    setStatus('')
    setFailed(false)
    try {
      await w.call('configSet', { scope: 0, key: `options.${key}`, value })
      await w.refresh()
      setStatus(t('Сохранено'))
    } catch (error) {
      setFailed(true)
      setStatus(error instanceof Error ? error.message : String(error))
    } finally {
      setSaving(false)
    }
  }
  const value = Number(draft)
  const valid = Number.isInteger(value) && value >= 1 && value <= 1000
  return (
    <section className="agent-settings">
      <h4>{t('Команда агентов')}</h4>
      <p className="muted">{t('Глобальные настройки для всех проектов и чатов.')}</p>
      <fieldset className="settings-fields" disabled={saving || !w.project}>
        <label className="setting-toggle">
          <span>{t('Параллельные агенты')}</span>
          <input
            type="checkbox"
            role="switch"
            checked={!options.disable_multi_agent}
            onChange={(event) => void save('disable_multi_agent', !event.target.checked)}
          />
        </label>
        <p className="muted">
          {t('Отключение запрещает новые запуски; текущие задания могут завершиться.')}
        </p>
        <label className="form-label" htmlFor="agent-concurrency">
          {t('Максимум одновременных агентов')}
          <input
            id="agent-concurrency"
            type="number"
            min={1}
            max={1000}
            step={1}
            value={draft}
            aria-invalid={!valid}
            aria-describedby="agent-concurrency-help"
            onChange={(event) => setDraft(event.target.value)}
            onBlur={() => {
              if (valid && value !== limit) void save('max_concurrent_agents', value)
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter') event.currentTarget.blur()
            }}
          />
        </label>
        <p id="agent-concurrency-help" className="muted">
          {t(
            'По умолчанию 10 дополнительных агентов, без основного чата. Снижение лимита не останавливает текущие задания.'
          )}
        </p>
        {!valid && (
          <p className="error-text" role="alert">
            {t('Введите целое число от 1 до 1000.')}
          </p>
        )}
      </fieldset>
      {status && (
        <p role="status" className={`settings-status ${failed ? '' : 'success'}`}>
          {status}
        </p>
      )}
    </section>
  )
}
