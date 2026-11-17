import { useEffect, useRef, useState, type ReactNode } from 'react'
import {
  Bot,
  Command,
  FolderOpen,
  ListTodo,
  MessageSquare,
  Palette,
  Plus,
  Puzzle,
  Settings2,
  TerminalSquare,
  X
} from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'
import { pickReturnProjectTarget } from '../designer/designHandoff'

export function ProjectRail({
  workspace: w,
  children,
  onCommands,
  onSettings,
  onJobs,
  onTools,
  onPlugins,
  customAgentsOpen = false,
  onCustomAgents,
  compact = false,
  expanded = false,
  onToggle,
  onExpandSidebar
}: {
  workspace: Workspace
  children: ReactNode
  onCommands: () => void
  onSettings: () => void
  onJobs: () => void
  onTools: () => void
  onPlugins: () => void
  customAgentsOpen?: boolean
  onCustomAgents?: () => void
  compact?: boolean
  expanded?: boolean
  onToggle?: () => void
  onExpandSidebar?: () => void
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [reopenTick, setReopenTick] = useState(0)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const wasCompact = useRef(compact)
  const prevSelection = useRef({ projectId: w.project?.id, sessionId: w.session?.id })
  const reopenFlyoutRef = useRef(false)
  const switchingFromDesignerRef = useRef(false)
  useEffect(() => {
    if (wasCompact.current && !compact) {
      reopenFlyoutRef.current = false
      setOpen(false)
    }
    wasCompact.current = compact
  }, [compact])
  useEffect(() => {
    const changed =
      prevSelection.current.projectId !== w.project?.id ||
      prevSelection.current.sessionId !== w.session?.id
    prevSelection.current = { projectId: w.project?.id, sessionId: w.session?.id }
    if (reopenFlyoutRef.current && w.project?.kind !== 'designer') {
      if (!switchingFromDesignerRef.current) reopenFlyoutRef.current = false
      setOpen(true)
      return
    }
    if (!switchingFromDesignerRef.current) reopenFlyoutRef.current = false
    if (changed) setOpen(false)
  }, [w.project?.id, w.session?.id, w.project?.kind, reopenTick])
  useEffect(() => {
    const toggle = () => setOpen((value) => !value)
    window.addEventListener('prime:projects-flyout', toggle)
    return () => window.removeEventListener('prime:projects-flyout', toggle)
  }, [])
  useEffect(() => {
    if (!open) return
    const pointer = (event: PointerEvent) => {
      const target = event.target as Element
      if (
        !root.current?.contains(target) &&
        !target.closest?.('[data-projects-toggle]') &&
        !target.closest?.('.prime-popover, .file-context-menu, .context-menu, .select-options, dialog')
      ) {
        reopenFlyoutRef.current = false
        setOpen(false)
      }
    }
    const keyboard = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return
      if (document.querySelector('dialog[open]')) return
      event.preventDefault()
      reopenFlyoutRef.current = false
      setOpen(false)
      trigger.current?.focus()
    }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', keyboard)
    return () => {
      document.removeEventListener('pointerdown', pointer)
      document.removeEventListener('keydown', keyboard)
    }
  }, [open])
  const isDesigner = w.project?.kind === 'designer'
  const isPersonalChat = w.project?.kind === 'chat'
  const handleProjectsClick = async () => {
    if (!isDesigner) {
      reopenFlyoutRef.current = false
      if (onToggle) onToggle()
      else setOpen((value) => !value)
      return
    }
    const isCompact = compact || !onToggle
    if (!isCompact) {
      if (onExpandSidebar) onExpandSidebar()
      else if (!expanded) onToggle()
    } else {
      reopenFlyoutRef.current = true
      switchingFromDesignerRef.current = true
    }
    try {
      const target = pickReturnProjectTarget(
        w.projects,
        w.preferences.recent,
        w.project?.kind,
        w.preferences.activeProject
      )
      if (target?.kind === 'project') {
        await w.selectProject(target.project)
      } else if (target?.kind === 'path') {
        await w.openProject(target.path)
      } else {
        await w.openChat()
      }
    } finally {
      if (isCompact) {
        switchingFromDesignerRef.current = false
        setReopenTick((tick) => tick + 1)
      }
    }
  }
  return (
    <div className="project-rail" ref={root}>
      <button
        ref={trigger}
        className={`icon ${!customAgentsOpen && (open || (expanded && !isDesigner && !isPersonalChat)) ? 'active' : ''}`}
        title={t('Панель проектов')}
        aria-label={t('Панель проектов')}
        aria-expanded={open || expanded}
        onClick={() => void handleProjectsClick()}
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
        className={`icon ${!customAgentsOpen && isPersonalChat ? 'active' : ''}`}
        title={t('Чат без проекта')}
        aria-label={t('Чат без проекта')}
        aria-pressed={!customAgentsOpen && isPersonalChat}
        disabled={w.info.runtime !== 'ready'}
        onClick={() => void w.openChat()}
      >
        <MessageSquare size={18} />
      </button>
      <button
        className={`icon ${!customAgentsOpen && isDesigner ? 'active' : ''}`}
        title={t('Дизайнер')}
        aria-label={t('Дизайнер')}
        aria-pressed={!customAgentsOpen && isDesigner}
        disabled={w.info.runtime !== 'ready'}
        onClick={() => void w.openDesigner()}
      >
        <Palette size={18} />
      </button>
      <button
        className={`icon ${customAgentsOpen ? 'active' : ''}`}
        title={t('Свои агенты')}
        aria-label={t('Свои агенты')}
        aria-pressed={customAgentsOpen}
        disabled={w.info.runtime !== 'ready'}
        onClick={onCustomAgents}
      >
        <Bot size={18} />
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
        title={t('Фоновые задачи')}
        aria-label={t('Фоновые задачи')}
        disabled={w.info.runtime !== 'ready'}
        onClick={onJobs}
      >
        <ListTodo size={18} />
      </button>
      <button
        className="icon"
        title={t('Инструменты проекта')}
        aria-label={t('Инструменты проекта')}
        disabled={!w.project || isDesigner}
        onClick={onTools}
      >
        <TerminalSquare size={18} />
      </button>
      <button className="icon" title={t('Плагины')} aria-label={t('Плагины')} onClick={onPlugins}>
        <Puzzle size={18} />
      </button>
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
              reopenFlyoutRef.current = false
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
