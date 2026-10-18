import { useRef, useState, type RefObject } from 'react'
import { Check, Search, Plus } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { useModelCapabilities, type ModelCapability } from './useModelCapabilities'

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
  const { models, loading, error: loadError } = useModelCapabilities(w)
  const [query, setQuery] = useState('')
  const [kind, setKind] = useState('large')
  const [saving, setSaving] = useState(false)
  const [saveError, setError] = useState('')
  const error = saveError || loadError
  const search = useRef<HTMLInputElement>(null)
  const selected = w.config.models?.[kind]
  const filtered = models.filter((model) =>
    `${model.name} ${model.model_id} ${model.provider_id} ${model.provider_name || ''}`
      .toLowerCase()
      .includes(query.toLowerCase())
  )
  const groups = new Map<string, { name: string; models: ModelCapability[] }>()
  for (const model of filtered) {
    const group = groups.get(model.provider_id) || {
      name: model.provider_name || model.provider_id,
      models: []
    }
    group.models.push(model)
    groups.set(model.provider_id, group)
  }
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
        {[...groups.entries()].map(([providerId, group]) => (
          <div className="model-picker-group" role="group" aria-label={group.name} key={providerId}>
            <div className="model-picker-provider">
              <span className="provider-monogram">{group.name.slice(0, 1).toUpperCase()}</span>
              {group.name}
            </div>
            {group.models.map((model) => {
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
                  <span className="model-description">
                    <strong>{model.name || model.model_id}</strong>
                    {model.context_window ? (
                      <small>{Math.round(model.context_window / 1000)}k</small>
                    ) : null}
                  </span>
                  {active && <Check size={16} />}
                </button>
              )
            })}
          </div>
        ))}
      </div>
      <button className="picker-footer" onClick={setup}>
        <Plus size={15} />
        {t('Подключить провайдера')}
      </button>
    </Popover>
  )
}
