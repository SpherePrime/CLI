import { shortcutLabel } from '../../../../shared/keybindings'
import { useShortcuts } from '../shortcuts/shortcutStore'
import { useI18n } from '../../i18n'
import '../../styles/jobs.css'
import { FolderOpen, Settings2, Puzzle, TerminalSquare, ListChecks, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import { SidebarChats } from './SidebarChats'
export function Sidebar({
  workspace: w,
  onSettings,
  onPlugins,
  onRemote,
  onJobs,
  onWorkbench
}: {
  workspace: Workspace
  onSettings: () => void
  onPlugins: () => void
  onRemote: () => void
  onJobs: () => void
  onWorkbench: () => void
}) {
  const { t } = useI18n()
  const shortcuts = useShortcuts()
  const [activeJobs, setActiveJobs] = useState(0)
  useEffect(() => {
    if (w.info.runtime !== 'ready') return
    const load = () =>
      void window.prime
        .jobs({ action: 'list' })
        .then((jobs) =>
          setActiveJobs(
            jobs.filter((job) => ['queued', 'running', 'waiting'].includes(job.status)).length
          )
        )
        .catch(() => {})
    load()
    const timer = setInterval(() => {
      if (!document.hidden) load()
    }, 5000)
    return () => clearInterval(timer)
  }, [w.info.runtime])
  return (
    <aside className="sidebar">
      <div className="sidebar-scroll">
        <SidebarChats workspace={w} onRemote={onRemote} />
      </div>
      <footer className="sidebar-footer">
        <button onClick={onJobs} className="sidebar-jobs">
          <ListChecks size={16} />
          {t('Фоновые задачи')}
          {activeJobs > 0 && <span className="sidebar-badge">{activeJobs}</span>}
        </button>
        <button onClick={onWorkbench} disabled={!w.project}>
          <TerminalSquare size={16} />
          {t('Инструменты проекта')}
          <kbd className="footer-shortcut">{shortcutLabel(shortcuts.terminal)}</kbd>
        </button>
        <button onClick={onPlugins}>
          <Puzzle size={16} /> {t('Плагины')}
        </button>
        <button onClick={onSettings}>
          <Settings2 size={16} /> {t('Настройки')}
          <kbd className="footer-shortcut">{shortcutLabel(shortcuts.settings)}</kbd>
        </button>
      </footer>
    </aside>
  )
}
