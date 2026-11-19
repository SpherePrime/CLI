import { useEffect, useRef, useState } from 'react'
import { Gamepad2 } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { GameDevPanel } from './GameDevPanel'
import './gamedev.css'

export function GameDevButton({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const enabled = Boolean(w.gameDevEnabled)
  const disabled = !w.project || w.busy
  const statusText = enabled ? t('Включён') : t('Выключен')
  const tooltip = t(
    'Режим GameDev: добавляет автотест-ботов для прохождения игры, поиска багов и проверки баланса сложности'
  )

  useEffect(() => {
    setOpen(false)
  }, [w.project?.id, w.session?.id])

  async function toggle() {
    if (saving || disabled) return
    setSaving(true)
    try {
      await w.setGameDev(!enabled)
      setOpen(false)
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <button
        ref={anchor}
        type="button"
        className={`gamedev-chip ${enabled ? 'is-active' : ''}`}
        title={tooltip}
        aria-label={`${t('Режим GameDev')}: ${statusText}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        data-active={enabled ? 'true' : 'false'}
        disabled={disabled}
        onClick={() => setOpen((value) => !value)}
      >
        <Gamepad2 size={14} aria-hidden="true" />
        <span className="gamedev-chip-label">{t('GameDev')}</span>
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={360} label={t('Режим GameDev')}>
          <GameDevPanel
            enabled={enabled}
            saving={saving}
            onClose={() => setOpen(false)}
            onToggle={toggle}
          />
        </Popover>
      )}
    </>
  )
}
