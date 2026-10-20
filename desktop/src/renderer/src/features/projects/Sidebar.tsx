import { useI18n } from '../../i18n'
import '../../styles/jobs.css'
import {
  FolderOpen,
  Plus,
  Settings2,
  Puzzle,
  TerminalSquare,
  ListChecks,
  X
} from 'lucide-react'
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
  const [activeJobs, setActiveJobs] = useState(0)
  useEffect(() => {
    if (w.info.runtime !== 'ready') return
    const load = () =>
      void window.prime
        .jobs({ action: 'list' })
        .then((jobs) => setActiveJobs(jobs.filter((job) => ['queued', 'running', 'waiting'].includes(job.status)).length))
        .catch(() => {})
    load()
    const timer = setInterval(load, 5000)
    return () => clearInterval(timer)
  }, [w.info.runtime])
  return (
    <aside className="sidebar">
      <div className="sidebar-actions">
        <button
          className="new-chat"
          disabled={!w.project}
          title={`${t('Новый чат')} · Ctrl N`}
          onClick={() => void w.createSession()}
        >
          <Plus size={16} /> {t('Новый чат')}
        </button>
        <button
          className="open-project"
          onClick={() => void w.openProject()}
          disabled={w.loading || w.info.runtime !== 'ready'}
        >
          <FolderOpen size={15} /> {t('Открыть проект')}
        </button>
      </div>
      <div className="sidebar-scroll">
        <SidebarChats workspace={w} onRemote={onRemote} />
        {w.preferences.recent.filter((path) => !w.projects.some((project) => project.path === path))
          .length > 0 && (
          <>
            <div className="section-caption recent-caption">{t('НЕДАВНИЕ')}</div>
            {w.preferences.recent
              .filter((path) => !w.projects.some((project) => project.path === path))
              .slice(0, 6)
              .map((path) => (
                <div key={path} className="recent-row">
                  <button
                    className="recent-project"
                    title={path}
                    onClick={() => void w.openProject(path)}
                  >
                    <FolderOpen size={14} />
                    <span>{path.split(/[\\/]/).pop()}</span>
                  </button>
                  <button
                    className="icon small recent-remove"
                    aria-label={t('Удалить из недавних')}
                    title={t('Удалить из недавних')}
                    onClick={() =>
                      void w.savePreferences({
                        recent: w.preferences.recent.filter((item) => item !== path)
                      })
                    }
                  >
                    <X size={12} />
                  </button>
                </div>
              ))}
          </>
        )}
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
          <kbd className="footer-shortcut">Ctrl `</kbd>
        </button>
        <button onClick={onPlugins}>
          <Puzzle size={16} /> {t('Плагины')}
        </button>
        <button onClick={onSettings}>
          <Settings2 size={16} /> {t('Настройки')}
          <kbd className="footer-shortcut">Ctrl ,</kbd>
        </button>
      </footer>
    </aside>
  )
}
