import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  Folder,
  FolderOpen,
  MessageSquare,
  Search,
  Server,
  X,
  Pin,
  Plus
} from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Project, Session } from '../../../../shared/contracts'
import type { Workspace } from '../../app/useWorkspace'

export function SidebarChats({
  workspace: w,
  onRemote
}: {
  workspace: Workspace
  onRemote: () => void
}) {
  const { t } = useI18n()
  const [search, setSearch] = useState('')
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null)
  const [renaming, setRenaming] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [otherSessions, setOtherSessions] = useState<Record<string, Session[]>>({})
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const projectIds = w.projects.map((project) => project.id).join('|')
  useEffect(() => {
    let cancelled = false
    async function load() {
      const entries = await Promise.all(
        w.projects
          .filter((project) => project.id !== w.project?.id)
          .map(async (project) => {
            try {
              return [project.id, (await window.prime.call(project.id, 'sessions')) as Session[]]
            } catch {
              return [project.id, [] as Session[]]
            }
          })
      )
      if (!cancelled) setOtherSessions(Object.fromEntries(entries))
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [projectIds, w.project?.id])
  useEffect(() => {
    if (!w.project?.id) return
    setExpanded((value) => (value[w.project!.id] ? value : { ...value, [w.project!.id]: true }))
  }, [w.project?.id])
  useEffect(() => {
    if (!menu) return
    const pointer = (event: PointerEvent) => {
      if ((event.target as Element).closest?.('.context-menu')) return
      setMenu(null)
    }
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenu(null)
    }
    document.addEventListener('pointerdown', pointer)
    document.addEventListener('keydown', key)
    return () => {
      document.removeEventListener('pointerdown', pointer)
      document.removeEventListener('keydown', key)
    }
  }, [menu])
  const needle = search.trim().toLocaleLowerCase()
  const personal = w.projects.find((project) => project.kind === 'chat')
  const folders = w.projects.filter((project) => project.kind !== 'chat')
  const recentFolders = w.preferences.recent.filter(
    (path) =>
      !w.projects.some((project) => project.path === path) &&
      (!needle || path.toLocaleLowerCase().includes(needle))
  )
  const sessionsOf = (project: Project) => {
    const sessions = project.id === w.project?.id ? w.sessions : otherSessions[project.id] || []
    const pinned = w.preferences.pinned
    return [...sessions]
      .filter((session) => !session.parent_session_id)
      .sort((a, b) => {
        const pin = Number(pinned.includes(b.id)) - Number(pinned.includes(a.id))
        return pin || (b.updated_at || 0) - (a.updated_at || 0)
      })
  }
  const openChatSession = async (project: Project, session: Session) => {
    if (project.id !== w.project?.id) await w.selectProject(project)
    await w.selectSession(session)
  }
  const toggle = (id: string, open: boolean) =>
    setExpanded((value) => ({ ...value, [id]: !open }))
  const menuSession = menu ? w.sessions.find((session) => session.id === menu.id) : null
  const renderSessions = (project: Project, active: boolean) => {
    const sessions = sessionsOf(project)
    if (!sessions.length) return null
    return sessions.map((session) => (
      <div
        key={session.id}
        className={`session-row ${session.id === w.session?.id && active ? 'active' : ''}`}
        onContextMenu={(event) => {
          if (!active) return
          event.preventDefault()
          setMenu({ id: session.id, x: event.clientX, y: event.clientY })
        }}
      >
        {renaming === session.id ? (
          <form
            onSubmit={async (event) => {
              event.preventDefault()
              try {
                await w.call('renameSession', {
                  sessionId: session.id,
                  session: { ...session, title }
                })
                setRenaming(null)
                await w.refresh()
              } catch (error) {
                w.errorHandler(error)
              }
            }}
          >
            <input
              autoFocus
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Escape') setRenaming(null)
              }}
            />
          </form>
        ) : (
          <button
            className="session-link"
            onClick={() => void openChatSession(project, session)}
          >
            <MessageSquare size={14} />
            <span>{session.title || t('Новый чат')}</span>
            {session.is_busy && <span className="dot pulse" />}
            {w.preferences.pinned.includes(session.id) && <Pin size={11} />}
          </button>
        )}
        {active && (
          <button
            className="icon small row-delete"
            aria-label={t('Удалить чат')}
            title={t('Удалить чат')}
            onClick={() => void w.deleteSession(session.id)}
          >
            <X size={14} />
          </button>
        )}
      </div>
    ))
  }
  return (
    <>
      <div className="section-caption">
        {t('ПРОЕКТЫ')}
        <span className="section-caption-actions">
          <button
            className="icon small"
            aria-label={t('Новый проект')}
            title={t('Новый проект')}
            onClick={() => void w.openProject()}
            disabled={w.loading || w.info.runtime !== 'ready'}
          >
            <Plus size={14} />
          </button>
          <button
            className="icon small"
            aria-label={t('SSH-серверы')}
            title={t('SSH-серверы')}
            onClick={onRemote}
            disabled={w.info.runtime !== 'ready'}
          >
            <Server size={14} />
          </button>
        </span>
      </div>
      <div className="project-search-box">
        <Search size={14} />
        <input
          aria-label={t('Поиск проектов')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Название проекта…')}
        />
        {search && (
          <button
            className="icon small"
            aria-label={t('Закрыть поиск')}
            onClick={() => setSearch('')}
          >
            <X size={12} />
          </button>
        )}
      </div>
      {personal && (
        <div className="project-group">
          <div className="project-heading">
            <button
              className={`project-item ${w.project?.id === personal.id ? 'selected' : ''}`}
              aria-expanded={Boolean(expanded[personal.id]) || w.project?.id === personal.id}
              onClick={() => {
                const open = Boolean(expanded[personal.id]) || w.project?.id === personal.id
                toggle(personal.id, open)
                if (w.project?.id !== personal.id) void w.openChat()
              }}
            >
              <MessageSquare size={15} />
              <span>{t('Личные чаты')}</span>
              <span className={`dot ${personal.connection === 'ready' ? 'green' : 'amber'}`} />
            </button>
            <button
              className="icon small project-close"
              title={t('Закрыть личные чаты')}
              aria-label={t('Закрыть личные чаты')}
              onClick={() => void w.closeProject(personal.id)}
            >
              <X size={12} />
            </button>
          </div>
          {(expanded[personal.id] || w.project?.id === personal.id) &&
            renderSessions(personal, w.project?.id === personal.id)}
        </div>
      )}
      {folders
        .filter((project) => !needle || project.path.toLocaleLowerCase().includes(needle))
        .map((project) => {
          const active = w.project?.id === project.id
          const open = Boolean(expanded[project.id]) || active
          return (
            <div key={project.id} className={`project-group ${active ? 'active-group' : ''}`}>
              <div className="project-heading">
                <button
                  className={`project-item ${active ? 'selected' : ''}`}
                  aria-expanded={open}
                  onClick={() => toggle(project.id, open)}
                >
                  {open ? <FolderOpen size={15} /> : <Folder size={15} />}
                  <span
                    title={
                      project.remote
                        ? `${project.remote.user}@${project.remote.host}:${project.remote.root}`
                        : project.path
                    }
                  >
                    {project.remote
                      ? `${project.remote.name} · ${project.remote.root.split('/').pop() || '/'}`
                      : project.path.split(/[\\/]/).pop()}
                  </span>
                  <span className={`dot ${project.connection === 'ready' ? 'green' : 'amber'}`} />
                </button>
                <button
                  className="icon small project-close"
                  title={t('Закрыть проект')}
                  aria-label={t('Закрыть проект {value0}', { value0: project.path })}
                  onClick={() => void w.closeProject(project.id)}
                >
                  <X size={12} />
                </button>
              </div>
              {open && renderSessions(project, active)}
            </div>
          )
        })}
      {recentFolders.length > 0 && (
        <>
          <div className="section-caption recent-caption">{t('НЕДАВНИЕ')}</div>
          {recentFolders.slice(0, 6).map((path) => (
            <div key={path} className="recent-row">
              <button
                className="recent-project"
                title={path}
                onClick={() => void w.openProject(path)}
              >
                <Folder size={14} />
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
      {!w.projects.length && !recentFolders.length && (
        <div className="sidebar-empty">{t('Откройте папку, чтобы начать работу с проектом')}</div>
      )}
      {menu && menuSession && createPortal(
        <div
          className="context-menu fly"
          style={{ top: menu.y, left: menu.x }}
          onMouseDown={(event) => event.stopPropagation()}
        >
          <button
            onClick={() => {
              void w.savePreferences({
                pinned: w.preferences.pinned.includes(menuSession.id)
                  ? w.preferences.pinned.filter((id) => id !== menuSession.id)
                  : [...w.preferences.pinned, menuSession.id]
              })
              setMenu(null)
            }}
          >
            <Pin size={14} />
            {w.preferences.pinned.includes(menuSession.id) ? t('Открепить') : t('Закрепить')}
          </button>
          <button
            onClick={() => {
              setTitle(menuSession.title)
              setRenaming(menuSession.id)
              setMenu(null)
            }}
          >
            {t('Переименовать')}
          </button>
        </div>,
        document.body
      )}
    </>
  )
}
