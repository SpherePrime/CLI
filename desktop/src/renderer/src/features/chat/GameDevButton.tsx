import { useState } from 'react'
import { Gamepad2 } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import './gamedev.css'

export function GameDevButton({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [saving, setSaving] = useState(false)
  const enabled = Boolean(w.gameDevEnabled)
  const disabled = !w.project || w.busy || saving
  const label = enabled ? t('Выключить режим GameDev') : t('Включить режим GameDev')
  const tooltip = t(
    'Режим GameDev: добавляет автотест-ботов для прохождения игры, поиска багов и проверки баланса сложности'
  )

  async function toggle() {
    if (disabled) return
    setSaving(true)
    try {
      await w.setGameDev(!enabled)
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setSaving(false)
    }
  }

  return (
    <button
      type="button"
      className={`gamedev-chip ${enabled ? 'is-active' : ''}`}
      title={tooltip}
      aria-label={label}
      aria-pressed={enabled}
      aria-busy={saving || undefined}
      disabled={disabled}
      onClick={() => void toggle()}
    >
      <Gamepad2 size={14} aria-hidden="true" />
      <span className="gamedev-chip-label">{t('GameDev')}</span>
    </button>
  )
}
