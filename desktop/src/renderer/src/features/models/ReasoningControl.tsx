import { useEffect, useState } from 'react'
import { Brain } from 'lucide-react'
import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ReasoningControl({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const selected = w.config.models?.large
  const [capability, setCapability] = useState<any>(null)
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    let active = true
    setCapability(null)
    if (w.project && selected)
      void w
        .call('models')
        .then((result) => {
          if (active)
            setCapability(
              (Array.isArray(result) ? result : result.models || []).find(
                (item: any) =>
                  item.provider_id === selected.provider && item.model_id === selected.model
              )
            )
        })
        .catch(w.errorHandler)
    return () => {
      active = false
    }
  }, [w.project?.id, selected?.provider, selected?.model])
  if (!capability?.can_reason || !capability.reasoning_modes?.length) return null
  const labels: Record<string, string> = {
    off: t('Выключены'),
    low: t('Низкий'),
    medium: t('Средний'),
    high: t('Высокий'),
    xhigh: t('Очень высокий'),
    max: t('Максимальный')
  }
  const mode =
    selected.reasoning_mode ||
    (selected.think
      ? 'medium'
      : selected.reasoning_effort === 'none'
        ? 'off'
        : selected.reasoning_effort) ||
    'off'
  return (
    <div className="reasoning-control" title={t('Уровень размышлений')}>
      <Brain size={14} />
      <Select
        label={t('Уровень размышлений')}
        value={mode}
        disabled={saving || w.busy || w.project?.connection !== 'ready'}
        options={capability.reasoning_modes.map((value: string) => ({
          value,
          label: labels[value] || value
        }))}
        onChange={(value) => {
          setSaving(true)
          void w
            .call('reasoning', { scope: 0, model_type: 'large', mode: value })
            .then(w.refresh)
            .catch(w.errorHandler)
            .finally(() => setSaving(false))
        }}
      />
    </div>
  )
}
