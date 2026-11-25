import { useEffect, useRef, useState } from 'react'
import { BadgeCheck } from 'lucide-react'
import { Popover } from '../../components/Popover'
import type { Workspace } from '../../app/useWorkspace'
import { useI18n } from '../../i18n'
import { StrictVerifyPanel } from './StrictVerifyPanel'
import './strictVerify.css'

export function StrictVerifyButton({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const anchor = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const enabled = Boolean(w.strictVerifyEnabled)
  const disabled = !w.project || w.busy
  const statusText = enabled ? t('Включена') : t('Выключена')
  const tooltip = t('Обязательный независимый ревьюер проверяет результат до идеала.')

  useEffect(() => {
    setOpen(false)
  }, [w.project?.id, w.session?.id])

  async function toggle() {
    if (saving || disabled) return
    setSaving(true)
    try {
      await w.setStrictVerify(!enabled)
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
        className={`strict-verify-chip${enabled ? ' is-active' : ''}`}
        title={`${t('Строгая проверка')}: ${statusText} · ${tooltip}`}
        aria-label={`${t('Строгая проверка')}: ${statusText}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        data-active={enabled ? 'true' : 'false'}
        disabled={disabled}
        onClick={() => setOpen((value) => !value)}
      >
        <BadgeCheck size={15} aria-hidden="true" />
        {enabled && <span className="strict-verify-chip-dot" aria-hidden="true" />}
      </button>
      {open && (
        <Popover
          anchor={anchor}
          close={() => setOpen(false)}
          width={336}
          label={t('Строгая проверка')}
        >
          <StrictVerifyPanel
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
