import { useRef, useState } from 'react'
import { Folder, FolderOpen, MessageSquare, Plus, Search, X } from 'lucide-react'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function ProjectPicker({ workspace: w }: { workspace: Workspace }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const anchor = useRef<HTMLButtonElement>(null)
  const needle = search.trim().toLocaleLowerCase()
  const folders = w.projects.filter(
    (project) =>
      project.kind !== 'chat' &&
      (!needle ||
        project.path.toLocaleLowerCase().includes(needle) ||
        (project.remote?.name || '').toLocaleLowerCase().includes(needle))
  )
  const recents = w.preferences.recent
    .filter((path) => !w.projects.some((project) => project.path === path))
    .filter((path) => !needle || path.toLocaleLowerCase().includes(needle))
    .slice(0, 8)
  const personal = w.projects.find((project) => project.kind === 'chat')
  return (
    <>
      <button
        ref={anchor}
        className="project-picker-trigger"
        aria-label={t('Выбрать проект')}
        aria-expanded={open}
        disabled={w.loading || w.info.runtime !== 'ready'}
        onClick={() => {
          setSearch('')
          setOpen(!open)
        }}
      >
        <FolderOpen size={15} />
        <span>{t('Выбрать проект')}</span>
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={280}>
          <div className="project-picker" role="menu" aria-label={t('Выбрать проект')}>
            <div className="picker-search">
              <Search size={14} />
              <input
                autoFocus
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
            <div className="picker-list">
              {folders.map((project) => (
                <button
                  key={project.id}
                  role="menuitem"
                  className={w.project?.id === project.id ? 'current' : ''}
                  onClick={async () => {
                    setOpen(false)
                    await w.selectProject(project)
                  }}
                >
                  <FolderOpen size={15} />
                  <span title={project.path}>
                    {project.remote
                      ? `${project.remote.name} · ${project.remote.root.split('/').pop() || '/'}`
                      : project.path.split(/[\\/]/).pop()}
                  </span>
                  {w.project?.id === project.id && (
                    <small className="picker-current">{t('Сейчас')}</small>
                  )}
                </button>
              ))}
              {recents.map((path) => (
                <button
                  key={path}
                  role="menuitem"
                  onClick={async () => {
                    setOpen(false)
                    await w.openProject(path)
                  }}
                >
                  <Folder size={15} />
                  <span title={path}>{path.split(/[\\/]/).pop()}</span>
                </button>
              ))}
              {!folders.length && !recents.length && (
                <p className="picker-empty">{t('Ничего не найдено')}</p>
              )}
            </div>
            <button
              role="menuitem"
              onClick={async () => {
                setOpen(false)
                await w.openProject()
              }}
            >
              <Plus size={15} />
              <span>{t('Новый проект')}</span>
            </button>
            <button
              role="menuitem"
              className={w.project?.id === personal?.id ? 'current' : ''}
              onClick={async () => {
                setOpen(false)
                await w.openChat()
              }}
            >
              <MessageSquare size={15} />
              <span>{t('Работать без проекта')}</span>
              {w.project?.id === personal?.id && (
                <small className="picker-current">{t('Сейчас')}</small>
              )}
            </button>
          </div>
        </Popover>
      )}
    </>
  )
}
