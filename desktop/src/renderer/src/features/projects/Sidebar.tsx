import { useI18n } from '../../i18n'
import { PrimeLogo } from '../../components/PrimeLogo'
import { Modal } from '../../components/Modal'
import {
  ChevronDown,
  FolderOpen,
  MessageSquare,
  Plus,
  Search,
  Settings2,
  Puzzle,
  Command,
  X,
  Pin,
  MoreHorizontal
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
export function Sidebar({
  workspace: w,
  onSettings,
  onPlugins
}: {
  workspace: Workspace
  onSettings: () => void
  onPlugins: () => void
}) {
  const { t } = useI18n()
  const [search, setSearch] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)
  useEffect(() => {
    setSearch('')
    setSearchOpen(false)
  }, [w.project?.id])
  const [menu, setMenu] = useState<string | null>(null)
  const [renaming, setRenaming] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [deleting, setDeleting] = useState<string | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const cancelDelete = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    setDeleting(null)
    setDeleteError('')
  }, [w.project?.id])
  const sessions = w.sessions
    .filter(
      (session) =>
        !session.parent_session_id && session.title.toLowerCase().includes(search.toLowerCase())
    )
    .sort(
      (a, b) =>
        Number(w.preferences.pinned.includes(b.id)) - Number(w.preferences.pinned.includes(a.id)) ||
        (b.updated_at || 0) - (a.updated_at || 0)
    )
  return (
    <aside className="sidebar">
      <div className="brand">
        <PrimeLogo />
        <strong>
          Prime<span className="brand-word-muted">Desktop</span>
        </strong>
      </div>
      <div className="sidebar-actions">
        <button className="new-chat" disabled={!w.project} onClick={() => void w.createSession()}>
          <Plus size={17} /> {t('Новый чат')}
          <kbd>Ctrl N</kbd>
        </button>
        <button
          className="quiet"
          onClick={() => void w.openProject()}
          disabled={w.loading || w.info.runtime !== 'ready'}
        >
          <FolderOpen size={16} /> {t('Открыть проект')}
        </button>
      </div>
      <button
        className="personal-chats"
        onClick={() => void w.openChat()}
        disabled={w.info.runtime !== 'ready'}
      >
        <MessageSquare size={16} />
        {t('Чат без проекта')}
      </button>
      <div className="sidebar-scroll">
        <div className="section-caption">
          {t('РАБОЧИЕ ПРОСТРАНСТВА')}{' '}
          <button
            className="icon small"
            aria-label={t('Добавить проект')}
            onClick={() => void w.openProject()}
          >
            <Plus size={14} />
          </button>
        </div>
        {w.projects.map((project) => (
          <div key={project.id} className="project-group">
            <div className="project-heading">
              <button
                className={`project-item ${w.project?.id === project.id ? 'selected' : ''}`}
                onClick={() => void w.selectProject(project)}
              >
                <ChevronDown size={14} />
                <FolderOpen size={15} />
                <span title={project.kind === 'chat' ? t('Личные чаты') : project.path}>
                  {project.kind === 'chat' ? t('Личные чаты') : project.path.split(/[\\/]/).pop()}
                </span>
                <span className={`dot ${project.connection === 'ready' ? 'green' : 'amber'}`} />
              </button>
              {project.kind !== 'chat' && (
                <button
                  className="icon small"
                  title={t('Закрыть проект')}
                  aria-label={t('Закрыть проект {value0}', { value0: project.path })}
                  onClick={() => void w.closeProject(project.id)}
                >
                  <X size={12} />
                </button>
              )}
            </div>
            {w.project?.id === project.id && (
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
                {sessions.map((session) => (
                  <div
                    key={session.id}
                    className={`session-row ${session.id === w.session?.id ? 'active' : ''}`}
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
                        onClick={() => void w.selectSession(session)}
                      >
                        <MessageSquare size={14} />
                        <span>{session.title || t('Новый чат')}</span>
                        {session.is_busy && <span className="dot pulse" />}
                        {w.preferences.pinned.includes(session.id) && <Pin size={11} />}
                      </button>
                    )}
                    <button
                      className="icon small row-menu"
                      aria-label={t('Действия: {value0}', { value0: session.title })}
                      onClick={() => setMenu(menu === session.id ? null : session.id)}
                    >
                      <MoreHorizontal size={15} />
                    </button>
                    {menu === session.id && (
                      <div className="context-menu">
                        <button
                          onClick={() => {
                            setTitle(session.title)
                            setRenaming(session.id)
                            setMenu(null)
                          }}
                        >
                          {t('Переименовать')}
                        </button>
                        <button
                          onClick={() => {
                            void w.savePreferences({
                              pinned: w.preferences.pinned.includes(session.id)
                                ? w.preferences.pinned.filter((id) => id !== session.id)
                                : [...w.preferences.pinned, session.id]
                            })
                            setMenu(null)
                          }}
                        >
                          {t('Закрепить / открепить')}
                        </button>
                        <button
                          className="danger"
                          onClick={() => {
                            setDeleting(session.id)
                            setDeleteError('')
                            setMenu(null)
                          }}
                        >
                          {t('Удалить чат')}
                        </button>
                      </div>
                    )}
                  </div>
                ))}
                {!sessions.length && (
                  <p className="sidebar-empty">{t('Здесь появятся ваши чаты')}</p>
                )}
              </>
            )}
          </div>
        ))}
        {!w.projects.length && (
          <div className="sidebar-empty">{t('Откройте папку, чтобы начать работу с проектом')}</div>
        )}
        {w.preferences.recent.filter((path) => !w.projects.some((project) => project.path === path))
          .length > 0 && (
          <>
            <div className="section-caption recent-caption">{t('НЕДАВНИЕ')}</div>
            {w.preferences.recent
              .filter((path) => !w.projects.some((project) => project.path === path))
              .slice(0, 6)
              .map((path) => (
                <button
                  key={path}
                  className="recent-project"
                  title={path}
                  onClick={() => void w.openProject(path)}
                >
                  <FolderOpen size={14} />
                  <span>{path.split(/[\\/]/).pop()}</span>
                </button>
              ))}
          </>
        )}
      </div>
      <footer className="sidebar-footer">
        <button onClick={onPlugins}>
          <Puzzle size={16} /> {t('Плагины')}
        </button>
        <button onClick={onSettings}>
          <Settings2 size={16} /> {t('Настройки')}
          <span className="footer-shortcut">⌘</span>
        </button>
      </footer>
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
    </aside>
  )
}
