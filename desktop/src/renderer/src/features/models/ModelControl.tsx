import { useEffect, useRef, useState } from 'react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { ModelPicker } from './ModelPicker'
import { ReasoningPopover } from './ReasoningPopover'
import { currentReasoningMode, reasoningLabel } from './reasoning'
import { findCapability, useModelCapabilities } from './useModelCapabilities'

export function ModelControl({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState<'reasoning' | 'models' | null>(null)
  const [saving, setSaving] = useState(false)
  const { models } = useModelCapabilities(w)
  const selected = w.config?.models?.large
  const capability = findCapability(models, selected)
  const mode = currentReasoningMode(selected, capability)
  const modelName =
    capability?.name || selected?.model || w.agent?.model?.name || t('Выбрать модель')
  useEffect(() => {
    const toggle = () => setOpen((value) => (value === 'models' ? null : 'models'))
    window.addEventListener('prime:models', toggle)
    return () => window.removeEventListener('prime:models', toggle)
  }, [])
  const setReasoning = (value: string) => {
    setSaving(true)
    void w
      .call('reasoning', { scope: 0, model_type: 'large', mode: value })
      .then(w.refresh)
      .catch(w.errorHandler)
      .finally(() => setSaving(false))
  }
  return (
    <>
      <button
        ref={anchor}
        className="model-control"
        aria-label={t('Модель и уровень размышлений')}
        aria-haspopup="dialog"
        aria-expanded={open !== null}
        disabled={!w.project}
        onClick={() => setOpen((value) => (value ? null : 'reasoning'))}
      >
        <span className="model-control-name">{modelName}</span>
        <span className="model-control-level">{reasoningLabel(mode, t)}</span>
      </button>
      {open === 'reasoning' && (
        <ReasoningPopover
          anchor={anchor}
          close={() => setOpen(null)}
          modelName={modelName}
          mode={mode}
          disabled={saving || w.busy || w.project?.connection !== 'ready'}
          onChange={setReasoning}
          onReset={() => setReasoning('default')}
          onOpenModels={() => setOpen('models')}
        />
      )}
      {open === 'models' && (
        <ModelPicker
          workspace={w}
          anchor={anchor}
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
