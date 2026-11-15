import { useMemo, useRef, useState } from 'react'
import { ArrowUpRight, Folder, FolderOpen, FolderPlus, LoaderCircle, Search, Server, X } from 'lucide-react'
import type { Project } from '../../../../shared/contracts'
import type { Workspace } from '../../app/useWorkspace'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import { projectName } from '../projects/projectName'
import {
  filterRegularProjects,
  filterRegularRecentPaths,
  formatDesignPreviewText,
  type DesignPayload
} from './designHandoff'
import './designer.css'

export function DesignHandoffDialog({
  workspace: w,
  design,
  close
}: {
  workspace: Workspace
  design: DesignPayload
  close: () => void
}) {
  const { t } = useI18n()
  const [search, setSearch] = useState('')
  const [pendingKey, setPendingKey] = useState<string | null>(null)
  const [error, setError] = useState('')
  const searchInput = useRef<HTMLInputElement>(null)
  const submitting = pendingKey !== null

  const needle = search.trim().toLocaleLowerCase()

  const previewText = useMemo(() => formatDesignPreviewText(design.text), [design.text])

  const openProjects = useMemo(() => {
    return filterRegularProjects(w.projects).filter((project) => {
      if (!needle) return true
      const label = (projectName(project) || '').toLocaleLowerCase()
      return (
        label.includes(needle) ||
        project.path.toLocaleLowerCase().includes(needle) ||
        (project.remote?.name || '').toLocaleLowerCase().includes(needle)
      )
    })
  }, [w.projects, needle])

  const recentPaths = useMemo(() => {
    return filterRegularRecentPaths(w.preferences.recent, w.projects).filter((path) => {
      if (!needle) return true
      return path.toLocaleLowerCase().includes(needle)
    })
  }, [w.preferences.recent, w.projects, needle])

  async function selectTarget(key: string, target: { project?: Project; path?: string }) {
    if (submitting) return
    setPendingKey(key)
    setError('')
    try {
      const ok = await w.implementDesign(target, design)
      if (ok) {
        close()
      } else {
        setError(t('Не удалось передать дизайн в выбранный проект.'))
      }
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setPendingKey(null)
    }
  }

  return (
    <Modal
      title={t('Сделать этот дизайн')}
      close={() => {
        if (!submitting) close()
      }}
      className="design-handoff-modal"
      initialFocus={searchInput}
    >
      <div className="design-handoff" aria-busy={submitting}>
        <p className="design-handoff-lead">
          {t('Выберите проект, в котором агент создаст новый чат и начнёт реализацию этого дизайна.')}
        </p>

        <div className="design-handoff-preview">
          <div className="design-handoff-preview-head">
            <strong title={design.title}>{design.title}</strong>
            {design.images.length > 0 && (
              <span className="design-handoff-badge">
                {t('Макетов: {count}', { count: design.images.length })}
              </span>
            )}
          </div>
          {previewText ? (
            <p className="design-handoff-snippet">{previewText}</p>
          ) : (
            <p className="design-handoff-snippet">
              {t('Текстовая спецификация дизайна будет передана в выбранный проект.')}
            </p>
          )}
          {design.images.length > 0 && (
            <div className="design-handoff-thumbs">
              {design.images.map((image, index) => (
                <img
                  key={`${image.name}-${index}`}
                  src={`data:${image.mimeType};base64,${image.base64}`}
                  alt={t('Макет дизайна {index}', { index: index + 1 })}
                  loading="lazy"
                />
              ))}
            </div>
          )}
        </div>

        <div className="design-handoff-search">
          <Search size={14} aria-hidden="true" />
          <input
            ref={searchInput}
            aria-label={t('Поиск проектов')}
            value={search}
            disabled={submitting}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Найти проект…')}
          />
          {search && (
            <button
              type="button"
              className="icon small"
              aria-label={t('Закрыть поиск')}
              disabled={submitting}
              onClick={() => setSearch('')}
            >
              <X size={12} />
            </button>
          )}
        </div>

        <div className="design-handoff-list" role="list" aria-label={t('Выбрать проект')}>
          {openProjects.map((project) => {
            const label = projectName(project) || project.path
            const subtitle = project.remote
              ? `${project.remote.user}@${project.remote.host}`
              : project.path.replace(/[\\/][^\\/]+[\\/]?$/, '') || project.path
            const isPending = pendingKey === project.id
            return (
              <button
                key={project.id}
                type="button"
                role="listitem"
                className="design-handoff-project"
                disabled={submitting}
                onClick={() => void selectTarget(project.id, { project })}
              >
                <span className="design-handoff-project-icon" aria-hidden="true">
                  {project.remote ? <Server size={16} /> : <FolderOpen size={16} />}
                </span>
                <span className="design-handoff-project-info">
                  <strong title={label}>{label}</strong>
                  <small title={project.path}>{subtitle}</small>
                </span>
                {isPending ? (
                  <LoaderCircle size={16} className="spin design-handoff-arrow" />
                ) : (
                  <ArrowUpRight size={16} className="design-handoff-arrow" />
                )}
              </button>
            )
          })}

          {recentPaths.map((path) => {
            const label = path.split(/[\\/]/).pop() || path
            const isPending = pendingKey === path
            return (
              <button
                key={path}
                type="button"
                role="listitem"
                className="design-handoff-project"
                disabled={submitting}
                onClick={() => void selectTarget(path, { path })}
              >
                <span className="design-handoff-project-icon" aria-hidden="true">
                  <Folder size={16} />
                </span>
                <span className="design-handoff-project-info">
                  <strong title={label}>{label}</strong>
                  <small title={path}>{path}</small>
                </span>
                {isPending ? (
                  <LoaderCircle size={16} className="spin design-handoff-arrow" />
                ) : (
                  <ArrowUpRight size={16} className="design-handoff-arrow" />
                )}
              </button>
            )
          })}

          {!openProjects.length && !recentPaths.length && (
            <p className="design-handoff-empty">
              {needle
                ? t('Подходящие проекты не найдены')
                : t('Нет доступных проектов. Создайте новый проект или откройте папку.')}
            </p>
          )}
        </div>

        <p className="design-handoff-hint">
          {submitting
            ? t('Передаём дизайн в проект…')
            : t('Создастся новый чат в режиме «Работа» с описанием и прикреплёнными макетами.')}
        </p>

        {error && (
          <p className="design-handoff-error" role="alert">
            {error}
          </p>
        )}

        <div className="design-handoff-actions">
          <button
            type="button"
            disabled={submitting}
            onClick={() => {
              close()
              void w.openProject()
            }}
          >
            <FolderPlus size={15} />
            <span>{t('Новый проект')}</span>
          </button>
          <button
            type="button"
            disabled={submitting}
            onClick={() => {
              close()
              void w.openFolder()
            }}
          >
            <FolderOpen size={15} />
            <span>{t('Открыть папку')}</span>
          </button>
        </div>
      </div>
    </Modal>
  )
}
