import { useRef, useState, type RefObject } from 'react'
import { ArrowLeft, Clock3, Search, Plus, Star } from 'lucide-react'
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
import { OrchestratorSettings } from './OrchestratorSettings'
import {
  resolveRoleDisplay,
  resolveRoleReasoning,
  roleReasoningOptions,
  type OrchestratorRole
} from './orchestrator'

export type ModelPickerTab = 'large' | 'small' | 'orchestrator'

export function ModelPicker({
  workspace: w,
  anchor,
  close,
  setup,
  initialTab = 'large'
}: {
  workspace: Workspace
  anchor: RefObject<HTMLElement | null>
  close: () => void
  setup: () => void
  initialTab?: ModelPickerTab
}) {
  const { t } = useI18n()
  const { models, loading, error: loadError } = useModelCapabilities(w)
  const [query, setQuery] = useState('')
  const [kind, setKind] = useState<ModelPickerTab>(initialTab)
  const [pickingRole, setPickingRole] = useState<OrchestratorRole | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setError] = useState('')
  const error = saveError || loadError
  const search = useRef<HTMLInputElement>(null)
  const sessionId = realSessionId(w.session)
  const sessionModel =
    kind === 'large' && sessionId
      ? (w.config?.session_models as Record<string, { provider: string; model: string }> | undefined)?.[
          sessionId
        ]
      : undefined
  const plannerDisplay = resolveRoleDisplay('planner', w.config, sessionId, models)
  const executorDisplay = resolveRoleDisplay('executor', w.config, sessionId, models)
  const selected =
    kind === 'orchestrator' && pickingRole
      ? pickingRole === 'planner'
        ? plannerDisplay.ref
        : executorDisplay.ref
      : sessionModel ?? w.config?.models?.[kind]
  const favorites = w.preferences.favoriteModels || []
  const favoriteIdentities = new Set(favorites.map(modelIdentity))
  const groups = buildModelGroups(models, w.preferences.recentModels, favorites, query)
  const showModelList = kind !== 'orchestrator' || pickingRole !== null
  const finalReview = w.config?.orchestrator?.final_review !== false

  const switchTab = (next: ModelPickerTab) => {
    setKind(next)
    setPickingRole(null)
    setError('')
  }

  return (
    <Popover anchor={anchor} close={close} width={420} align="end" label={t('Выбрать модель')}>
      {kind === 'orchestrator' && pickingRole ? (
        <div className="model-picker-heading orchestrator-picking-heading">
          <button
            type="button"
            className="orchestrator-back-btn"
            onClick={() => {
              setPickingRole(null)
              setQuery('')
              setError('')
            }}
          >
            <ArrowLeft size={14} aria-hidden="true" />
            <span>
              {t('Назад')} ·{' '}
              {pickingRole === 'planner' ? t('Выбор планировщика') : t('Выбор исполнителя')}
            </span>
          </button>
        </div>
      ) : (
        <div className="model-picker-heading">
          <strong>{t('Выбрать модель')}</strong>
          <div className="segmented" role="tablist" aria-label={t('Режим выбора модели')}>
            <button
              type="button"
              role="tab"
              aria-selected={kind === 'large'}
              className={kind === 'large' ? 'active' : ''}
              onClick={() => switchTab('large')}
            >
              {t('Основная')}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={kind === 'small'}
              className={kind === 'small' ? 'active' : ''}
              onClick={() => switchTab('small')}
            >
              {t('Быстрая')}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={kind === 'orchestrator'}
              className={kind === 'orchestrator' ? 'active' : ''}
              onClick={() => switchTab('orchestrator')}
            >
              {t('Оркестратор')}
            </button>
          </div>
        </div>
      )}

      {showModelList ? (
        <>
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
            {loading && !models.length && (
              <p className="picker-note muted">{t('Загружаем каталог моделей…')}</p>
            )}
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
                      <span className="provider-monogram">
                        {(group.name || group.id || '?').slice(0, 1).toUpperCase()}
                      </span>
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
                        key={`${group.id}:${modelIdentity(reference)}`}
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
                          if (model.locked) return
                          setSaving(true)
                          setError('')
                          try {
                            if (kind === 'orchestrator' && pickingRole) {
                              const currentDisplay =
                                pickingRole === 'planner' ? plannerDisplay : executorDisplay
                              const allowed = roleReasoningOptions(pickingRole, model)
                              const nextReasoning = allowed.includes(currentDisplay.reasoning)
                                ? currentDisplay.reasoning
                                : resolveRoleReasoning(pickingRole, undefined, model)
                              await w.setOrchestrator({
                                [pickingRole]: {
                                  provider: model.provider_id,
                                  model: model.model_id,
                                  ...(nextReasoning ? { reasoning_mode: nextReasoning } : {})
                                }
                              })
                              await w.savePreferences({
                                recentModels: recordRecentModel(w.preferences.recentModels, reference)
                              })
                              setPickingRole(null)
                              setQuery('')
                              return
                            }
                            const perSession = kind === 'large' && sessionId
                            await w.call('model', {
                              modelType: kind,
                              model: { provider: model.provider_id, model: model.model_id },
                              ...(perSession ? { sessionId } : {})
                            })
                            if (kind === 'large' && w.orchestratorEnabled) {
                              await w.setOrchestrator({ enabled: false })
                            }
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
        </>
      ) : (
        <>
          {error && (
            <p className="error-text picker-note" role="alert">
              {error}
            </p>
          )}
          <OrchestratorSettings
            enabled={w.orchestratorEnabled}
            finalReview={finalReview}
            planner={{
              name: plannerDisplay.name,
              provider: plannerDisplay.provider,
              reasoning: plannerDisplay.reasoning,
              reasoningOptions: plannerDisplay.reasoningOptions,
              hasModel: Boolean(plannerDisplay.ref)
            }}
            executor={{
              name: executorDisplay.name,
              provider: executorDisplay.provider,
              reasoning: executorDisplay.reasoning,
              reasoningOptions: executorDisplay.reasoningOptions,
              hasModel: Boolean(executorDisplay.ref)
            }}
            disabled={saving || w.busy}
            onToggleEnabled={async (next) => {
              setSaving(true)
              setError('')
              try {
                await w.setOrchestrator({ enabled: next })
              } catch (error: any) {
                setError(error?.message || String(error))
              } finally {
                setSaving(false)
              }
            }}
            onToggleFinalReview={async (next) => {
              setSaving(true)
              setError('')
              try {
                await w.setOrchestrator({ finalReview: next })
              } catch (error: any) {
                setError(error?.message || String(error))
              } finally {
                setSaving(false)
              }
            }}
            onChangeReasoning={async (role, level) => {
              const display = role === 'planner' ? plannerDisplay : executorDisplay
              if (!display.ref) return
              setSaving(true)
              setError('')
              try {
                await w.setOrchestrator({
                  [role]: {
                    provider: display.ref.provider,
                    model: display.ref.model,
                    reasoning_mode: level
                  }
                })
              } catch (error: any) {
                setError(error?.message || String(error))
              } finally {
                setSaving(false)
              }
            }}
            onPickModel={(role) => {
              setQuery('')
              setError('')
              setPickingRole(role)
            }}
          />
        </>
      )}
    </Popover>
  )
}
