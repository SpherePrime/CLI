import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { FolderOpen, MessageSquare, Search, Server, X, Pin, Plus } from 'lucide-react'
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
  const [searchOpen, setSearchOpen] = useState(false)
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null)
  const [renaming, setRenaming] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [otherSessions, setOtherSessions] = useState<Record<string, Session[]>>({})
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
  const ordered = [...w.projects].sort(
    (a, b) => Number(b.kind === 'chat') - Number(a.kind === 'chat')
  )
  const personalExists = w.projects.some((project) => project.kind === 'chat')
  const sessionsOf = (project: Project) => {
    const sessions = project.id === w.project?.id ? w.sessions : otherSessions[project.id] || []
    const pinned = w.preferences.pinned
    return [...sessions]
      .filter((session) => !session.parent_session_id)
      .filter((session) => !needle || (session.title || '').toLocaleLowerCase().includes(needle))
      .sort((a, b) => {
        const pin = Number(pinned.includes(b.id)) - Number(pinned.includes(a.id))
        return pin || (b.updated_at || 0) - (a.updated_at || 0)
      })
  }
  const openChatSession = async (project: Project, session: Session) => {
    if (project.id !== w.project?.id) await w.selectProject(project)
    await w.selectSession(session)
  }
  const menuSession = menu ? w.sessions.find((session) => session.id === menu.id) : null
  return (
    <>
      <div className="section-caption">
        {t('ЧАТЫ И ПРОЕКТЫ')}
        <span className="section-caption-actions">
          <button
            className="icon small"
            aria-label={t('Найти чат')}
            title={t('Найти чат')}
            aria-expanded={searchOpen}
            onClick={() => {
              setSearchOpen(!searchOpen)
              setSearch('')
            }}
          >
            <Search size={14} />
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
      {searchOpen && (
        <div className="chat-search-box">
          <Search size={14} />
          <input
            autoFocus
            aria-label={t('Поиск чатов')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Название чата…')}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                setSearchOpen(false)
                setSearch('')
              }
            }}
          />
          <button
            className="icon small"
            aria-label={t('Закрыть поиск')}
            onClick={() => {
              setSearchOpen(false)
              setSearch('')
            }}
          >
            <X size={12} />
          </button>
        </div>
      )}
      {!personalExists && (
        <div className="project-group">
          <button
            className="project-item personal-open"
            onClick={() => void w.openChat()}
            disabled={w.info.runtime !== 'ready'}
          >
            <Plus size={14} />
            <MessageSquare size={15} />
            <span>{t('Личные чаты')}</span>
          </button>
        </div>
      )}
      {ordered.map((project) => {
        const active = w.project?.id === project.id
        const sessions = sessionsOf(project)
        return (
          <div key={project.id} className={`project-group ${active ? 'active-group' : ''}`}>
            <div className="project-heading">
              <button
                className={`project-item ${active ? 'selected' : ''}`}
                onClick={() =>
                  project.kind === 'chat' ? void w.openChat() : void w.selectProject(project)
                }
              >
                {project.remote ? <Server size={15} /> : <FolderOpen size={15} />}
                <span
                  title={
                    project.kind === 'chat'
                      ? t('Личные чаты')
                      : project.remote
                        ? `${project.remote.user}@${project.remote.host}:${project.remote.root}`
                        : project.path
                  }
                >
                  {project.kind === 'chat'
                    ? t('Личные чаты')
                    : project.remote
                      ? `${project.remote.name} · ${project.remote.root.split('/').pop() || '/'}`
                      : project.path.split(/[\\/]/).pop()}
                </span>
                {sessions.length > 0 && <span className="project-count">{sessions.length}</span>}
                <span className={`dot ${project.connection === 'ready' ? 'green' : 'amber'}`} />
              </button>
              <button
                className="icon small project-close"
                title={project.kind === 'chat' ? t('Закрыть личные чаты') : t('Закрыть проект')}
                aria-label={
                  project.kind === 'chat'
                    ? t('Закрыть личные чаты')
                    : t('Закрыть проект {value0}', { value0: project.path })
                }
                onClick={() => void w.closeProject(project.id)}
              >
                <X size={12} />
              </button>
            </div>
            {sessions.map((session) => (
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
            ))}
          </div>
        )
      })}
      {!w.projects.length && (
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
