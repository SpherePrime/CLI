import { useEffect, useRef, useState, type RefObject } from 'react'
import { Check, Search, Plus } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ModelPicker({
  workspace: w,
  anchor,
  close,
  setup
}: {
  workspace: Workspace
  anchor: RefObject<HTMLElement | null>
  close: () => void
  setup: () => void
}) {
  const { t } = useI18n()
  const [models, setModels] = useState<any[]>([])
  const [query, setQuery] = useState('')
  const [kind, setKind] = useState('large')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const search = useRef<HTMLInputElement>(null)
  useEffect(() => {
    let current = true
    void w
      .call('models')
      .then((result) => {
        if (current) setModels(Array.isArray(result) ? result : result.models || [])
      })
      .catch((error) => {
        if (current) setError(error.message)
      })
      .finally(() => {
        if (current) setLoading(false)
      })
    return () => {
      current = false
    }
  }, [w.project?.id])
  const selected = w.config.models?.[kind]
  const capability = models.find(
    (item) => item.provider_id === selected?.provider && item.model_id === selected?.model
  )
  const modes: string[] = capability?.reasoning_modes || capability?.reasoning_levels || []
  const filtered = models.filter((model) =>
    `${model.name} ${model.model_id} ${model.provider_id}`
      .toLowerCase()
      .includes(query.toLowerCase())
  )
  return (
    <Popover anchor={anchor} close={close} width={390} label={t('Выбрать модель')}>
      <div className="model-picker-heading">
        <strong>{t('Выбрать модель')}</strong>
        <div className="segmented">
          <button className={kind === 'large' ? 'active' : ''} onClick={() => setKind('large')}>
            {t('Основная')}
          </button>
          <button className={kind === 'small' ? 'active' : ''} onClick={() => setKind('small')}>
            {t('Быстрая')}
          </button>
        </div>
      </div>
      <label className="search-field model-search">
        <Search size={15} />
        <input
          ref={search}
          autoFocus
          aria-label={t('Найти модель или провайдера')}
          placeholder={t('Найти модель или провайдера')}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown') {
              event.preventDefault()
              document.querySelector<HTMLElement>('.model-picker-list [role=option]')?.focus()
            }
          }}
        />
      </label>
      {error && (
        <p className="error-text picker-note" role="alert">
          {error}
        </p>
      )}
      <div
        className="model-picker-list"
        role="listbox"
        aria-label={t('Модели')}
        aria-busy={loading || saving}
        onKeyDown={(event) => {
          if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
            event.preventDefault()
            const buttons = Array.from(
              event.currentTarget.querySelectorAll<HTMLButtonElement>('[role=option]')
            )
            const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
            buttons[
              event.key === 'Home'
                ? 0
                : event.key === 'End'
                  ? buttons.length - 1
                  : (index + (event.key === 'ArrowUp' ? -1 : 1) + buttons.length) % buttons.length
            ]?.focus()
          }
        }}
      >
        {loading && <p className="picker-note muted">{t('Загружаем каталог моделей…')}</p>}
        {!loading && !filtered.length && (
          <p className="picker-note muted">
            {models.length
              ? t('Модели не найдены')
              : t('Подключите провайдера, чтобы увидеть модели')}
          </p>
        )}
        {filtered.map((model) => {
          const active =
            model.provider_id === selected?.provider && model.model_id === selected?.model
          return (
            <button
              role="option"
              aria-selected={active}
              disabled={saving || w.busy}
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
                  close()
                  anchor.current?.focus()
                } catch (error: any) {
                  setError(error.message)
                } finally {
                  setSaving(false)
                }
              }}
            >
              <span className="provider-monogram">
                {model.provider_id.slice(0, 1).toUpperCase()}
              </span>
              <span className="model-description">
                <strong>{model.name || model.model_id}</strong>
                <small>
                  {model.provider_id}
                  {model.context_window ? ` · ${Math.round(model.context_window / 1000)}k` : ''}
                </small>
              </span>
              {active && <Check size={16} />}
            </button>
          )
        })}
      </div>
      {capability?.can_reason && modes.length > 0 && (
        <div className="picker-reasoning">
          <span>{t('Глубина рассуждений')}</span>
          <Select
            label={t('Глубина рассуждений')}
            value={selected?.reasoning_mode || modes[0]}
            options={modes.map((value) => ({ value, label: value }))}
            onChange={(value) => {
              void w
                .call('reasoning', { scope: 0, model_type: kind, mode: value })
                .then(w.refresh)
                .catch(w.errorHandler)
            }}
          />
        </div>
      )}
      <button className="picker-footer" onClick={setup}>
        <Plus size={15} />
        {t('Подключить провайдера')}
      </button>
    </Popover>
  )
}
