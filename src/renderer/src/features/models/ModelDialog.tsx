import { useI18n } from '../../i18n'
import { useEffect, useState } from 'react'
import { Check, Search, Sparkles, Server, Plus } from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { Workspace } from '../../app/useWorkspace'
export function ModelDialog({
  workspace: w,
  close,
  setup
}: {
  workspace: Workspace
  close: () => void
  setup: () => void
}) {
  const { t } = useI18n()
  const [models, setModels] = useState<any[]>([])
  const [query, setQuery] = useState('')
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(true)
  const [kind, setKind] = useState('large')
  const [reasoning, setReasoning] = useState(w.config?.models?.large?.reasoning_mode || 'off')
  const [error, setError] = useState('')
  useEffect(() => {
    void w
      .call('models')
      .then((result) => setModels(Array.isArray(result) ? result : result.models || []))
      .catch((error) => setError(error.message))
      .finally(() => setLoading(false))
  }, [w.project?.id])
  const selected = w.config?.models?.[kind]
  const capability = models.find(
    (model) => model.provider_id === selected?.provider && model.model_id === selected?.model
  )
  const modes = capability?.reasoning_modes || capability?.reasoning_levels || []
  return (
    <Modal title={t('Модели и глубина рассуждений')} close={close}>
      <div className="segmented">
        <button className={kind === 'large' ? 'active' : ''} onClick={() => setKind('large')}>
          {t('Основная модель')}
        </button>
        <button className={kind === 'small' ? 'active' : ''} onClick={() => setKind('small')}>
          {t('Быстрая модель')}
        </button>
      </div>
      <label className="search-field">
        <Search size={16} />
        <input
          autoFocus
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t('Найти модель или провайдера')}
        />
      </label>
      {error && <p className="error-text">{error}</p>}
      <div className="model-list">
        {loading ? (
          <p className="muted">{t('Загружаем каталог моделей…')}</p>
        ) : (
          models
            .filter((model) =>
              `${model.provider_id} ${model.name} ${model.model_id}`
                .toLowerCase()
                .includes(query.toLowerCase())
            )
            .map((model) => (
              <button
                disabled={saving}
                className={`model-option ${selected?.model === model.model_id && selected?.provider === model.provider_id ? 'selected' : ''}`}
                key={`${model.provider_id}/${model.model_id}`}
                onClick={async () => {
                  setSaving(true)
                  try {
                    await w.call('model', {
                      modelType: kind,
                      model: { provider: model.provider_id, model: model.model_id }
                    })
                    await w.call('init')
                    await w.refresh()
                  } catch (error: any) {
                    setError(error.message)
                  } finally {
                    setSaving(false)
                  }
                }}
              >
                <span className="model-logo">
                  <Server size={17} />
                </span>
                <span>
                  <strong>{model.name || model.model_id}</strong>
                  <small>
                    {model.provider_id} · {Math.round((model.context_window || 0) / 1000)}
                    {t('k контекст')} {model.can_reason ? t('· рассуждения') : ''}
                  </small>
                </span>
                {selected?.model === model.model_id && selected?.provider === model.provider_id && (
                  <Check size={16} />
                )}
              </button>
            ))
        )}
        {!loading && !models.length && (
          <div className="empty-small">{t('Подключите провайдера, чтобы увидеть модели')}</div>
        )}
      </div>
      {capability?.can_reason && modes.length > 0 && (
        <label className="form-label">
          {t('Глубина рассуждений')}
          <select
            value={reasoning}
            onChange={async (event) => {
              const mode = event.target.value
              setReasoning(mode)
              try {
                await w.call('reasoning', { scope: 0, model_type: kind, mode })
                await w.call('agent').catch(() => {})
                await w.refresh()
              } catch (error: any) {
                setError(error.message)
              }
            }}
          >
            {modes.map((mode: string) => (
              <option value={mode} key={mode}>
                {mode}
              </option>
            ))}
          </select>
        </label>
      )}
      <button className="quiet full-width" onClick={setup}>
        <Plus size={15} /> {t('Подключить провайдера')}
      </button>
    </Modal>
  )
}
