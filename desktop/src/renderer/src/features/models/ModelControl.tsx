import { useEffect, useRef, useState } from 'react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { ContextMeter } from './ContextMeter'
import { ContextPopover } from './ContextPopover'
import { MaxReasoningDialog } from './MaxReasoningDialog'
import { needsMaxReasoningNotice } from './maxReasoningNotice'
import { ModelPicker, type ModelPickerTab } from './ModelPicker'
import { ReasoningPopover } from './ReasoningPopover'
import { currentReasoningMode, reasoningLabel } from './reasoning'
import { findCapability, useModelCapabilities } from './useModelCapabilities'
import {
  compactModelLabel,
  isOrchestratorActive,
  resolveContextWindow,
  resolveRoleDisplay
} from './orchestrator'
import { realSessionId } from '../../../../shared/pendingSession'
import type { ChatReasoningMode } from '../../../../shared/contracts'

export function ModelControl({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const contextAnchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState<'reasoning' | 'models' | 'context' | null>(null)
  const [pickerInitialTab, setPickerInitialTab] = useState<ModelPickerTab>('large')
  const [saving, setSaving] = useState(false)
  const [maxNotice, setMaxNotice] = useState(false)
  const { models } = useModelCapabilities(w)
  const sessionId = realSessionId(w.session)
  const sessionModel = sessionId
    ? (w.config?.session_models as Record<string, { provider: string; model: string; context_window?: number }> | undefined)?.[
        sessionId
      ]
    : undefined
  const selected = sessionModel ?? w.config?.models?.large
  const capability = findCapability(models, selected)
  const mode =
    w.reasoningMode === 'default' ? currentReasoningMode(undefined, capability) : w.reasoningMode
  const modelName =
    capability?.name || selected?.model || w.agent?.model?.name || t('Выбрать модель')
  const plannerDisplay = resolveRoleDisplay('planner', w.config, sessionId, models)
  const executorDisplay = resolveRoleDisplay('executor', w.config, sessionId, models)
  const orchestratorActive = isOrchestratorActive(
    w.orchestratorEnabled,
    w.preferences?.agentMode,
    w.project?.kind
  )
  const plannerShort = compactModelLabel(plannerDisplay.name) || t('Планировщик')
  const executorShort = compactModelLabel(executorDisplay.name) || t('Исполнитель')
  const orchestratorPairLabel = `${plannerShort} → ${executorShort}`
  const contextUsed = (w.session?.prompt_tokens || 0) + (w.session?.completion_tokens || 0)
  const contextWindow = resolveContextWindow({
    orchestratorActive,
    plannerRef: plannerDisplay.ref,
    plannerCapability: plannerDisplay.capability,
    selected,
    capability
  })
  useEffect(() => {
    const toggle = () => {
      setPickerInitialTab('large')
      setOpen((value) => (value === 'models' ? null : 'models'))
    }
    window.addEventListener('prime:models', toggle)
    return () => window.removeEventListener('prime:models', toggle)
  }, [])
  const setReasoning = (value: string) => {
    setSaving(true)
    void w
      .setReasoningMode(value as ChatReasoningMode)
      .catch(w.errorHandler)
      .finally(() => setSaving(false))
  }
  const chooseReasoning = (value: string) => {
    if (needsMaxReasoningNotice(value, mode, w.preferences?.skipMaxReasoningNotice)) {
      setOpen(null)
      setMaxNotice(true)
      return
    }
    setReasoning(value)
  }
  const enableMax = (skipNextTime: boolean) => {
    setMaxNotice(false)
    if (skipNextTime) void w.savePreferences({ skipMaxReasoningNotice: true }).catch(w.errorHandler)
    setReasoning('max')
  }
  const disableOrchestrator = () => {
    setSaving(true)
    void w
      .setOrchestrator({ enabled: false })
      .then(() => setOpen(null))
      .catch(w.errorHandler)
      .finally(() => setSaving(false))
  }
  return (
    <>
      {contextUsed > 0 && contextWindow > 0 && (
        <ContextMeter
          ref={contextAnchor}
          used={contextUsed}
          window={contextWindow}
          expanded={open === 'context'}
          onClick={() => setOpen((value) => (value === 'context' ? null : 'context'))}
        />
      )}
      {open === 'context' && (
        <ContextPopover
          workspace={w}
          anchor={contextAnchor}
          close={() => setOpen(null)}
          fallbackWindow={contextWindow}
        />
      )}
      <button
        ref={anchor}
        className={`model-control${orchestratorActive ? ' is-orchestrator' : ''}`}
        aria-label={
          orchestratorActive
            ? `${t('Оркестратор')}: ${orchestratorPairLabel}`
            : `${modelName} · ${t('Уровень размышлений')}: ${reasoningLabel(mode, t)}`
        }
        title={
          orchestratorActive
            ? `${t('Оркестратор')}: ${plannerDisplay.name || t('Планировщик')} → ${executorDisplay.name || t('Исполнитель')}`
            : `${modelName} · ${reasoningLabel(mode, t)}`
        }
        aria-haspopup="dialog"
        aria-expanded={open === 'reasoning' || open === 'models'}
        disabled={!w.project}
        onClick={() =>
          setOpen((value) => (value === 'reasoning' || value === 'models' ? null : 'reasoning'))
        }
      >
        <span className="model-control-name">
          {orchestratorActive ? t('Оркестратор') : modelName}
        </span>
        {orchestratorActive && (
          <span className="model-control-level">{orchestratorPairLabel}</span>
        )}
      </button>
      {open === 'reasoning' && (
        <ReasoningPopover
          anchor={anchor}
          close={() => setOpen(null)}
          modelName={modelName}
          mode={mode}
          disabled={saving || w.busy || w.project?.connection !== 'ready'}
          onChange={chooseReasoning}
          onReset={() => setReasoning('default')}
          onOpenModels={() => {
            setPickerInitialTab('large')
            setOpen('models')
          }}
          orchestrator={
            orchestratorActive
              ? {
                  enabled: true,
                  plannerName: plannerDisplay.name,
                  plannerReasoning: plannerDisplay.reasoning,
                  executorName: executorDisplay.name,
                  executorReasoning: executorDisplay.reasoning,
                  onOpenOrchestrator: () => {
                    setPickerInitialTab('orchestrator')
                    setOpen('models')
                  },
                  onDisable: disableOrchestrator
                }
              : undefined
          }
        />
      )}
      {maxNotice && <MaxReasoningDialog close={() => setMaxNotice(false)} confirm={enableMax} />}
      {open === 'models' && (
        <ModelPicker
          workspace={w}
          anchor={anchor}
          initialTab={pickerInitialTab}
          close={() => setOpen(null)}
          setup={() => {
            setOpen(null)
            window.dispatchEvent(new Event('prime:providers'))
          }}
        />
      )}
    </>
  )
}
