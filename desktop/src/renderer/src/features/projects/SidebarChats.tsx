import { sessionTitle } from '../../../../shared/sessionTitle'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
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
import { orderedProjectPaths } from './projectOrder'
import { useProjectReorder } from './useProjectReorder'
import './projectReorder.css'

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
      if (!cancelled) setOtherSessions((value) => ({ ...value, ...Object.fromEntries(entries) }))
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [projectIds, w.project?.id])
  const previousProject = useRef<{ id?: string; sessions: Session[] }>({ sessions: [] })
  useLayoutEffect(() => {
    const last = previousProject.current
    if (last.id && last.id !== w.project?.id)
      setOtherSessions((value) => ({ ...value, [last.id!]: last.sessions }))
    previousProject.current = { id: w.project?.id, sessions: w.sessions }
  }, [w.project?.id, w.sessions])
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
  const projectLabel = (project: Project) =>
    project.remote
      ? `${project.remote.name} · ${project.remote.root.split('/').pop() || '/'}`
      : project.path.split(/[\\/]/).pop()
  const projectPaths = orderedProjectPaths(
    w.preferences.recent,
    folders.map((project) => project.path),
    w.preferences.projectOrder
  )
  const reorder = useProjectReorder(projectPaths, (projectOrder) => {
    void w.savePreferences({ projectOrder })
  })
  const visited = projectPaths.map(
    (path) => folders.find((project) => project.path === path) ?? path
  ).filter((entry) => {
    const path = typeof entry === 'string' ? entry : entry.path
    return !needle || path.toLocaleLowerCase().includes(needle)
  })
  const sessionsOf = (project: Project) => {
    const sessions = project.id === w.project?.id ? w.sessions : otherSessions[project.id] || []
    const pinned = w.preferences.pinned
    return [...new Map(sessions.map((session) => [session.id, session])).values()]
      .filter((session) => !session.parent_session_id)
      .sort((a, b) => {
        const pin = Number(pinned.includes(b.id)) - Number(pinned.includes(a.id))
        return pin || (b.updated_at || 0) - (a.updated_at || 0)
      })
  }
  const deleteChatSession = async (project: Project, session: Session) => {
    try {
      if (project.id === w.project?.id) {
        await w.deleteSession(session.id)
        return
      }
      await window.prime.call(project.id, 'deleteSession', { sessionId: session.id })
      setOtherSessions((value) => ({
        ...value,
        [project.id]: (value[project.id] || []).filter((item) => item.id !== session.id)
      }))
    } catch (error) {
      w.errorHandler(error)
    }
  }
  const openChatSession = async (project: Project, session: Session) => {
    await w.openSession(project, session, otherSessions[project.id])
  }
  const recentChats = [personal, ...folders]
    .filter((project): project is Project => Boolean(project))
    .flatMap((project) => sessionsOf(project).map((session) => ({ project, session })))
    .filter(
      ({ session }) =>
        !needle || sessionTitle(session.title, t('Новый чат')).toLocaleLowerCase().includes(needle)
    )
    .sort((a, b) => (b.session.updated_at || 0) - (a.session.updated_at || 0))
    .slice(0, 8)
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
            <span>{sessionTitle(session.title, t('Новый чат'))}</span>
            {session.is_busy && <span className="dot pulse" />}
            {w.preferences.pinned.includes(session.id) && <Pin size={11} />}
          </button>
        )}
        <button
          className="icon small row-delete"
          aria-label={t('Удалить чат')}
          title={t('Удалить чат')}
          onClick={() => void deleteChatSession(project, session)}
        >
          <X size={14} />
        </button>
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
              aria-expanded={Boolean(expanded[personal.id])}
              onClick={() => {
                const open = Boolean(expanded[personal.id])
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
          {expanded[personal.id] && renderSessions(personal, w.project?.id === personal.id)}
        </div>
      )}
      {visited.map((entry) => {
        if (typeof entry === 'string')
          return (
            <div key={entry} className="project-group">
              <div {...reorder.headingProps(entry)}>
                <button
                  {...reorder.buttonProps(entry)}
                  className="project-item"
                  title={entry}
                  onClick={() => void w.openProject(entry)}
                >
                  <Folder size={15} />
                  <span>{entry.split(/[\\/]/).pop()}</span>
                </button>
                <button
                  className="icon small project-close"
                  aria-label={t('Удалить из недавних')}
                  title={t('Удалить из недавних')}
                  onClick={() =>
                    void w.savePreferences({
                      recent: w.preferences.recent.filter((item) => item !== entry),
                      projectOrder: projectPaths.filter((item) => item !== entry)
                    })
                  }
                >
                  <X size={12} />
                </button>
              </div>
            </div>
          )
        const project = entry
        const active = w.project?.id === project.id
        const open = Boolean(expanded[project.id])
        return (
          <div key={project.id} className={`project-group ${active ? 'active-group' : ''}`}>
            <div {...reorder.headingProps(project.path)}>
              <button
                {...reorder.buttonProps(project.path)}
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
                  {projectLabel(project)}
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
      {recentChats.length > 0 && (
        <>
          <div className="section-caption recent-caption">{t('НЕДАВНИЕ')}</div>
          {recentChats.map(({ project, session }) => (
            <div
              key={`${project.id}:${session.id}`}
              className={`session-row recent-chat ${
                session.id === w.session?.id && !expanded[project.id] ? 'active' : ''
              }`}
            >
              <button
                className="session-link"
                title={sessionTitle(session.title, t('Новый чат'))}
                onClick={() => void openChatSession(project, session)}
              >
                <MessageSquare size={14} />
                <span>{sessionTitle(session.title, t('Новый чат'))}</span>
                <small className="recent-chat-project">
                  {project.kind === 'chat' ? t('Личные чаты') : projectLabel(project)}
                </small>
                {session.is_busy && <span className="dot pulse" />}
              </button>
              <button
                className="icon small row-delete"
                aria-label={t('Удалить чат')}
                title={t('Удалить чат')}
                onClick={() => void deleteChatSession(project, session)}
              >
                <X size={14} />
              </button>
            </div>
          ))}
        </>
      )}
      {!w.projects.length && !visited.length && (
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
