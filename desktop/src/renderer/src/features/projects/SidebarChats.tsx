import { sessionTitle } from '../../../../shared/sessionTitle'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { Search, Server, X, Pin, Plus, MessagesSquare } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Project, Session } from '../../../../shared/contracts'
import type { Workspace } from '../../app/useWorkspace'
import { orderedProjectPaths } from './projectOrder'
import { useProjectReorder } from './useProjectReorder'
import { watchProjectSessions } from './projectSessionEvents'
import { ProjectCard } from './ProjectCard'
import { SidebarSession } from './SidebarSession'
import './projectNavigation.css'
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
  const [menu, setMenu] = useState<{ id: string; projectId: string; x: number; y: number } | null>(
    null
  )
  const [renaming, setRenaming] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [otherSessions, setOtherSessions] = useState<Record<string, Session[]>>({})
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const projectIds = w.projects.map((project) => project.id).join('|')
  useEffect(() => {
    return watchProjectSessions(
      w.projects.map((project) => project.id),
      window.prime,
      (projectId, sessions) => setOtherSessions((value) => ({ ...value, [projectId]: sessions }))
    )
  }, [projectIds])
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
  const recentChats = [personal]
    .filter((project): project is Project => Boolean(project))
    .flatMap((project) => sessionsOf(project).map((session) => ({ project, session })))
    .filter(
      ({ session }) =>
        !needle || sessionTitle(session.title, t('Новый чат')).toLocaleLowerCase().includes(needle)
    )

  const isWorking = (project: Project, session: Session) =>
    (project.id === w.project?.id && session.id === w.session?.id && w.busy) ||
    (otherSessions[project.id]?.find((item) => item.id === session.id)?.is_busy ?? session.is_busy)
  const toggle = (id: string, open: boolean) => setExpanded((value) => ({ ...value, [id]: !open }))
  const menuProject = menu ? w.projects.find((project) => project.id === menu.projectId) : null
  const menuSession = menuProject
    ? sessionsOf(menuProject).find((session) => session.id === menu?.id)
    : null
  const renameSession = async (project: Project, session: Session) => {
    if (!title.trim()) return
    try {
      await window.prime.call(project.id, 'renameSession', {
        sessionId: session.id,
        session: { ...session, title: title.trim() }
      })
      setRenaming(null)
      if (project.id === w.project?.id) await w.refresh()
      else
        setOtherSessions((value) => ({
          ...value,
          [project.id]: (value[project.id] || []).map((item) =>
            item.id === session.id ? { ...item, title: title.trim() } : item
          )
        }))
    } catch (error) {
      w.errorHandler(error)
    }
  }
  const createProjectChat = async (project?: Project) => {
    try {
      if (project) await w.selectProject(project)
      else await w.openChat()
      await w.createSession()
    } catch (error) {
      w.errorHandler(error)
    }
  }
  const matching = (session: Session) =>
    !needle || sessionTitle(session.title, t('Новый чат')).toLocaleLowerCase().includes(needle)
  const visited = projectPaths
    .map((path) => folders.find((project) => project.path === path) ?? path)
    .filter((entry) =>
      typeof entry === 'string'
        ? !needle || entry.toLocaleLowerCase().includes(needle)
        : !needle ||
          entry.path.toLocaleLowerCase().includes(needle) ||
          sessionsOf(entry).some(matching)
    )
  const renderSession = (project: Project, session: Session, recent = false) => (
    <SidebarSession
      key={session.id}
      session={session}
      personal={recent}
      active={project.id === w.project?.id && session.id === w.session?.id}
      busy={Boolean(isWorking(project, session))}
      pinned={w.preferences.pinned.includes(session.id)}
      renaming={renaming === session.id}
      title={title}
      onTitle={setTitle}
      onRename={() => renameSession(project, session)}
      onCancelRename={() => setRenaming(null)}
      onOpen={() => void openChatSession(project, session)}
      onDelete={() => void deleteChatSession(project, session)}
      onMenu={(x, y) => setMenu({ id: session.id, projectId: project.id, x, y })}
    />
  )
  const renderSessions = (project: Project) =>
    sessionsOf(project)
      .filter(
        (session) =>
          !needle || project.path.toLocaleLowerCase().includes(needle) || matching(session)
      )
      .map((session) => renderSession(project, session))
  return (
    <>
      <div className="section-caption navigation-caption">
        <span>
          {t('ПРОЕКТЫ')} <b>{projectPaths.length}</b>
        </span>
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
          aria-label={t('Поиск проектов и чатов')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={t('Найти проект или чат…')}
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
      <div className="project-card-list">
        {visited.map((entry) => {
          const closed = typeof entry === 'string'
          const project = closed ? undefined : entry
          const path = closed ? entry : project!.path
          const name = project ? projectLabel(project) || path : path.split(/[\\/]/).pop() || path
          const subtitle = project?.remote
            ? `${project.remote.user}@${project.remote.host}`
            : path.replace(/[\\/][^\\/]+[\\/]?$/, '') || path
          const open = Boolean(project && (needle || expanded[project.id]))
          return (
            <ProjectCard
              key={path}
              name={name}
              path={path}
              subtitle={closed ? t('Не открыт') : subtitle}
              active={project?.id === w.project?.id}
              open={open}
              closed={closed}
              remote={Boolean(project?.remote)}
              ready={project?.connection === 'ready'}
              count={project ? sessionsOf(project).length : undefined}
              headingProps={reorder.headingProps(path)}
              buttonProps={reorder.buttonProps(path)}
              onToggle={() => (project ? toggle(project.id, open) : void w.openProject(path))}
              onClose={() =>
                project
                  ? void w.closeProject(project.id)
                  : void w.savePreferences({
                      recent: w.preferences.recent.filter((item) => item !== path),
                      projectOrder: projectPaths.filter((item) => item !== path)
                    })
              }
              onNewChat={() => void createProjectChat(project)}
            >
              {project &&
                (sessionsOf(project).length ? (
                  renderSessions(project)
                ) : (
                  <button
                    className="project-empty-chat"
                    onClick={() => void createProjectChat(project)}
                  >
                    <Plus size={13} />
                    {t('Создать первый чат')}
                  </button>
                ))}
            </ProjectCard>
          )
        })}
      </div>
      {needle && !visited.length && !recentChats.length && (
        <div className="sidebar-empty">{t('Проекты и чаты не найдены')}</div>
      )}
      <section className="personal-chat-section" aria-label={t('Личные чаты')}>
        <div className="section-caption navigation-caption recent-caption">
          <span>
            <MessagesSquare size={13} />
            {t('ЛИЧНЫЕ ЧАТЫ')}
            <b>{personal ? sessionsOf(personal).length : 0}</b>
          </span>
          <button
            className="icon small"
            aria-label={t('Новый личный чат')}
            title={t('Новый личный чат')}
            disabled={w.loading || w.info.runtime !== 'ready'}
            onClick={() => void createProjectChat()}
          >
            <Plus size={14} />
          </button>
        </div>
        <p className="personal-chat-hint">{t('Без проекта')}</p>
        {recentChats.map(({ project, session }) => renderSession(project, session, true))}
        {!needle && !recentChats.length && (
          <button className="personal-empty-chat" onClick={() => void createProjectChat()}>
            <Plus size={14} />
            {t('Начать личный чат')}
          </button>
        )}
      </section>
      {!w.projects.length && !visited.length && (
        <div className="sidebar-empty">{t('Откройте папку, чтобы начать работу с проектом')}</div>
      )}
      {menu &&
        menuSession &&
        createPortal(
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
