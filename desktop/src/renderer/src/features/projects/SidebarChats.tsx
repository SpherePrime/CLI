import { useEffect, useRef, useState, Fragment } from 'react'
import {
  ChevronDown,
  FolderOpen,
  MessageSquare,
  Search,
  Server,
  X,
  Pin,
  MoreHorizontal,
  Plus
} from 'lucide-react'
import { useI18n } from '../../i18n'
import { Modal } from '../../components/Modal'
import { groupSessions } from '../../../../shared/workspaceTools'
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
  const [menu, setMenu] = useState<string | null>(null)
  const [renaming, setRenaming] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [deleting, setDeleting] = useState<string | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const cancelDelete = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    setSearch('')
    setSearchOpen(false)
    setDeleting(null)
    setDeleteError('')
  }, [w.project?.id])
  useEffect(() => {
    if (!menu) return
    const pointer = (event: PointerEvent) => {
      const target = event.target as Element
      if (target.closest?.('.context-menu') || target.closest?.('.row-menu')) return
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
  const groups = groupSessions(w.sessions, w.preferences.pinned, search)
  const groupLabels = {
    pinned: t('Закреплённые'),
    today: t('Сегодня'),
    yesterday: t('Вчера'),
    week: t('Последние 7 дней'),
    older: t('Ранее')
  }
  const ordered = [...w.projects].sort((a, b) => Number(b.kind === 'chat') - Number(a.kind === 'chat'))
  const personalExists = w.projects.some((project) => project.kind === 'chat')
  const openProjectGroup = (project: Project) => {
    if (project.kind === 'chat') {
      void w.openChat()
      return
    }
    void w.selectProject(project)
  }
  return (
    <>
      <div className="section-caption">
        {t('ЧАТЫ И ПРОЕКТЫ')}
        <span className="section-caption-actions">
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
        return (
          <div key={project.id} className={`project-group ${active ? 'open' : ''}`}>
            <div className="project-heading">
              <button
                className={`project-item ${active ? 'selected' : ''}`}
                onClick={() => openProjectGroup(project)}
              >
                <ChevronDown size={14} className={active ? '' : 'collapsed'} />
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
                <span className={`dot ${project.connection === 'ready' ? 'green' : 'amber'}`} />
              </button>
              <button
                className="icon small"
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
            {active && (
              <>
                <div className="chat-list-heading">
                  <span>{t('Чаты')}</span>
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
                {groups.map((group) => (
                  <Fragment key={group.id}>
                    <div className="chat-group-label">
                      <span>{groupLabels[group.id]}</span>
                      <span>{group.sessions.length}</span>
                    </div>
                    {group.sessions.map((session) => (
                      <ChatRow
                        key={session.id}
                        session={session}
                        workspace={w}
                        active={session.id === w.session?.id}
                        menuOpen={menu === session.id}
                        renaming={renaming === session.id}
                        title={title}
                        onToggleMenu={() => setMenu(menu === session.id ? null : session.id)}
                        onRenameStart={() => {
                          setTitle(session.title)
                          setRenaming(session.id)
                          setMenu(null)
                        }}
                        onRenameChange={setTitle}
                        onRenameSubmit={async () => {
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
                        onRenameCancel={() => setRenaming(null)}
                        onPin={() => {
                          void w.savePreferences({
                            pinned: w.preferences.pinned.includes(session.id)
                              ? w.preferences.pinned.filter((id) => id !== session.id)
                              : [...w.preferences.pinned, session.id]
                          })
                          setMenu(null)
                        }}
                        onDelete={() => {
                          setDeleting(session.id)
                          setDeleteError('')
                          setMenu(null)
                        }}
                      />
                    ))}
                  </Fragment>
                ))}
                {!groups.length && (
                  <p className="sidebar-empty">
                    {search ? t('Ничего не найдено') : t('Здесь появятся ваши чаты')}
                  </p>
                )}
              </>
            )}
          </div>
        )
      })}
      {!w.projects.length && (
        <div className="sidebar-empty">{t('Откройте папку, чтобы начать работу с проектом')}</div>
      )}
      {deleting && (
        <Modal
          title={t('Удалить чат и его историю?')}
          className="delete-chat-modal"
          initialFocus={cancelDelete}
          close={() => {
            if (!deleteBusy) setDeleting(null)
          }}
        >
          <div className="delete-chat-name">
            <MessageSquare size={18} />
            <strong>
              {w.sessions.find((session) => session.id === deleting)?.title || t('Новый чат')}
            </strong>
          </div>
          <p className="muted">
            {t('Чат и его сообщения будут удалены. Отменить это действие нельзя.')}
          </p>
          {deleteError && (
            <p role="alert" className="danger">
              {deleteError}
            </p>
          )}
          <div className="delete-chat-actions">
            <button ref={cancelDelete} disabled={deleteBusy} onClick={() => setDeleting(null)}>
              {t('Отмена')}
            </button>
            <button
              className="delete-chat-submit"
              disabled={deleteBusy}
              onClick={async () => {
                if (deleteBusy) return
                setDeleteBusy(true)
                setDeleteError('')
                try {
                  await w.deleteSession(deleting)
                  setDeleting(null)
                } catch (error) {
                  setDeleteError(error instanceof Error ? error.message : String(error))
                } finally {
                  setDeleteBusy(false)
                }
              }}
            >
              {deleteBusy ? t('Удаление…') : t('Удалить')}
            </button>
          </div>
        </Modal>
      )}
    </>
  )
}

function ChatRow({
  session,
  workspace: w,
  active,
  menuOpen,
  renaming,
  title,
  onToggleMenu,
  onRenameStart,
  onRenameChange,
  onRenameSubmit,
  onRenameCancel,
  onPin,
  onDelete
}: {
  session: Session
  workspace: Workspace
  active: boolean
  menuOpen: boolean
  renaming: boolean
  title: string
  onToggleMenu: () => void
  onRenameStart: () => void
  onRenameChange: (value: string) => void
  onRenameSubmit: () => Promise<void>
  onRenameCancel: () => void
  onPin: () => void
  onDelete: () => void
}) {
  const { t } = useI18n()
  return (
    <div className={`session-row ${active ? 'active' : ''}`}>
      {renaming ? (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void onRenameSubmit()
          }}
        >
          <input
            autoFocus
            value={title}
            onChange={(event) => onRenameChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') onRenameCancel()
            }}
          />
        </form>
      ) : (
        <button className="session-link" onClick={() => void w.selectSession(session)}>
          <MessageSquare size={14} />
          <span>{session.title || t('Новый чат')}</span>
          {session.is_busy && <span className="dot pulse" />}
          {w.preferences.pinned.includes(session.id) && <Pin size={11} />}
        </button>
      )}
      <button
        className="icon small row-menu"
        aria-label={t('Действия: {value0}', { value0: session.title })}
        onClick={onToggleMenu}
      >
        <MoreHorizontal size={15} />
      </button>
      {menuOpen && (
        <div className="context-menu">
          <button onClick={onRenameStart}>{t('Переименовать')}</button>
          <button onClick={onPin}>{t('Закрепить / открепить')}</button>
          <button className="danger" onClick={onDelete}>
            {t('Удалить чат')}
          </button>
        </div>
      )}
    </div>
  )
}
