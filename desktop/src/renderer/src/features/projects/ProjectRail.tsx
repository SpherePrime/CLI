import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Command, FolderOpen, MessageSquare, Plus, Settings2, X } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ProjectRail({
  workspace: w,
  children,
  onCommands,
  onSettings
}: {
  workspace: Workspace
  children: ReactNode
  onCommands: () => void
  onSettings: () => void
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    setOpen(false)
  }, [w.project?.id, w.session?.id])
  useEffect(() => {
    if (!open) return
    const pointer = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
        trigger.current?.focus()
      }
    }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', keyboard)
    return () => {
      document.removeEventListener('pointerdown', pointer)
      document.removeEventListener('keydown', keyboard)
    }
  }, [open])
  return (
    <div className="project-rail" ref={root}>
      <button
        ref={trigger}
        className={`icon ${open ? 'active-control' : ''}`}
        title={t('Панель проектов')}
        aria-label={t('Панель проектов')}
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <FolderOpen size={19} />
      </button>
      <button
        className="icon"
        title={t('Новый чат')}
        aria-label={t('Новый чат')}
        disabled={!w.project}
        onClick={() => void w.createSession()}
      >
        <Plus size={19} />
      </button>
      <button
        className="icon"
        title={t('Чат без проекта')}
        aria-label={t('Чат без проекта')}
        disabled={w.info.runtime !== 'ready'}
        onClick={() => void w.openChat()}
      >
        <MessageSquare size={18} />
      </button>
      <button
        className="icon"
        title={t('Быстрые действия')}
        aria-label={t('Быстрые действия')}
        onClick={onCommands}
      >
        <Command size={18} />
      </button>
      <span className="rail-spacer" />
      <button
        className="icon"
        title={t('Настройки')}
        aria-label={t('Настройки')}
        onClick={onSettings}
      >
        <Settings2 size={18} />
      </button>
      {open && (
        <div className="project-flyout">
          <button
            className="icon compact-panel-close"
            title={t('Закрыть')}
            aria-label={t('Закрыть')}
            onClick={() => {
              setOpen(false)
              trigger.current?.focus()
            }}
          >
            <X size={16} />
          </button>
          {children}
        </div>
      )}
    </div>
  )
}
