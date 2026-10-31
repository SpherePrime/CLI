import { useRef, useState, type RefObject } from 'react'
import { Clock3, Search, Plus, Star } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { useModelCapabilities } from './useModelCapabilities'
import { realSessionId } from '../../../../shared/pendingSession'
import { ModelPickerRow } from './ModelPickerRow'
import {
  buildModelGroups,
  modelIdentity,
  recordRecentModel,
  toggleFavoriteModel
} from './modelPreferences'

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
  const sessionId = realSessionId(w.session)
  const sessionModel =
    kind === 'large' && sessionId
      ? (w.config.session_models as Record<string, { provider: string; model: string }> | undefined)?.[
          sessionId
        ]
      : undefined
  const selected = sessionModel ?? w.config.models?.[kind]
  const favorites = w.preferences.favoriteModels || []
  const favoriteIdentities = new Set(favorites.map(modelIdentity))
  const groups = buildModelGroups(models, w.preferences.recentModels, favorites, query)
  return (
    <Popover anchor={anchor} close={close} width={390} align="end" label={t('Выбрать модель')}>
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
              document
                .querySelector<HTMLElement>('.model-picker-list [role=option]:not(:disabled)')
                ?.focus()
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
              event.currentTarget.querySelectorAll<HTMLButtonElement>(
                '[role=option]:not(:disabled)'
              )
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
        {loading && !models.length && <p className="picker-note muted">{t('Загружаем каталог моделей…')}</p>}
        {!loading && !groups.length && (
          <p className="picker-note muted">
            {models.length
              ? t('Модели не найдены')
              : t('Подключите провайдера, чтобы увидеть модели')}
          </p>
        )}
        {groups.map((group) => {
          const groupName =
            group.kind === 'recent'
              ? t('Недавние')
              : group.kind === 'favorites'
                ? t('Избранное')
                : group.name
          return (
            <div className="model-picker-group" role="group" aria-label={groupName} key={group.id}>
              <div className="model-picker-provider">
                {group.kind === 'recent' ? (
                  <Clock3 size={16} aria-hidden="true" />
                ) : group.kind === 'favorites' ? (
                  <Star size={16} aria-hidden="true" />
                ) : (
                  <span className="provider-monogram">{group.name.slice(0, 1).toUpperCase()}</span>
                )}
                {groupName}
              </div>
              {group.models.map((model) => {
                const active =
                  model.provider_id === selected?.provider && model.model_id === selected?.model
                const reference = { provider: model.provider_id, model: model.model_id }
                const favorite = favoriteIdentities.has(modelIdentity(reference))
                return (
                  <ModelPickerRow
                    model={model}
                    active={active}
                    favorite={favorite}
                    disabled={saving || w.busy}
                    favoriteDisabled={saving || (!favorite && favorites.length >= 100)}
                    showProvider={group.kind !== 'provider'}
                    key={modelIdentity(reference)}
                    toggleFavorite={async () => {
                      setSaving(true)
                      setError('')
                      try {
                        await w.savePreferences({
                          favoriteModels: toggleFavoriteModel(favorites, reference)
                        })
                      } catch (error: any) {
                        setError(error.message)
                      } finally {
                        setSaving(false)
                      }
                    }}
                    select={async () => {
                      setSaving(true)
                      setError('')
                      try {
                        const perSession = kind === 'large' && sessionId
                        await w.call('model', {
                          modelType: kind,
                          model: { provider: model.provider_id, model: model.model_id },
                          ...(perSession ? { sessionId } : {})
                        })
                        if (!perSession) await w.call('init')
                        await w.refresh()
                        await w.savePreferences({
                          recentModels: recordRecentModel(w.preferences.recentModels, reference)
                        })
                        close()
                        anchor.current?.focus()
                      } catch (error: any) {
                        setError(error.message)
                      } finally {
                        setSaving(false)
                      }
                    }}
                  />
                )
              })}
            </div>
          )
        })}
      </div>
      <button className="picker-footer" onClick={setup}>
        <Plus size={15} />
        {t('Подключить провайдера')}
      </button>
    </Popover>
  )
}
