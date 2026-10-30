import { ChevronRight, Folder, FolderOpen, Plus, Server, X } from 'lucide-react'
import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from 'react'
import { useI18n } from '../../i18n'

export function ProjectCard({
  name,
  path,
  subtitle,
  active,
  open,
  ready,
  remote,
  closed,
  count,
  headingProps,
  buttonProps,
  onToggle,
  onClose,
  onNewChat,
  children
}: {
  name: string
  path: string
  subtitle: string
  active: boolean
  open: boolean
  ready: boolean
  remote?: boolean
  closed?: boolean
  count?: number
  headingProps: HTMLAttributes<HTMLDivElement>
  buttonProps: ButtonHTMLAttributes<HTMLButtonElement>
  onToggle: () => void
  onClose: () => void
  onNewChat?: () => void
  children?: ReactNode
}) {
  const { t } = useI18n()
  const Icon = remote ? Server : open ? FolderOpen : Folder
  return (
    <section
      className={`project-group project-card ${active ? 'active-group' : ''}`}
      role="group"
      aria-label={t('Проект {name}', { name })}
    >
      <div {...headingProps}>
        <button
          {...buttonProps}
          className={`project-item ${active ? 'selected' : ''}`}
          title={path}
          aria-expanded={closed ? undefined : open}
          onClick={onToggle}
        >
          <span className="project-symbol">
            <Icon size={16} />
            {!closed && <i className={`project-connection ${ready ? 'ready' : 'pending'}`} />}
          </span>
          <span className="project-identity">
            <strong>{name}</strong>
            <small>{subtitle}</small>
          </span>
          <span className="project-count" title={t('Чатов в проекте')}>
            {count ?? '—'}
          </span>
          <ChevronRight size={13} className={`project-chevron ${open ? 'expanded' : ''}`} />
        </button>
        <button
          className="icon small project-close"
          title={t('Удалить проект')}
          aria-label={t('Удалить проект {value0}', { value0: path })}
          onClick={onClose}
        >
          <X size={12} />
        </button>
      </div>
      {open && (
        <div className="project-chat-area">
          <div className="project-chat-caption">
            <span>{t('ЧАТЫ ПРОЕКТА')}</span>
            <button
              className="icon small project-new-chat"
              aria-label={t('Новый чат в проекте {name}', { name })}
              title={t('Новый чат в этом проекте')}
              onClick={onNewChat}
            >
              <Plus size={13} />
            </button>
          </div>
          <div className="project-chat-list">{children}</div>
        </div>
      )}
    </section>
  )
}
