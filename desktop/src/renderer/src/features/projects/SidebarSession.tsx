import { MessageSquare, Pin, X } from 'lucide-react'
import type { HTMLAttributes } from 'react'
import type { Session } from '../../../../shared/contracts'
import { sessionTitle } from '../../../../shared/sessionTitle'
import { useI18n } from '../../i18n'
import { SessionActivity } from './SessionActivity'

export function SidebarSession({
  session,
  active,
  personal,
  busy,
  pinned,
  renaming,
  title,
  onTitle,
  onRename,
  onCancelRename,
  onOpen,
  onDelete,
  onMenu,
  rowProps,
  linkProps
}: {
  session: Session
  active: boolean
  personal: boolean
  busy: boolean
  pinned: boolean
  renaming: boolean
  title: string
  onTitle: (title: string) => void
  onRename: () => Promise<void>
  onCancelRename: () => void
  onOpen: () => void
  onDelete: () => void
  onMenu: (x: number, y: number) => void
  rowProps?: HTMLAttributes<HTMLDivElement>
  linkProps?: HTMLAttributes<HTMLElement>
}) {
  const { t } = useI18n()
  return (
    <div
      {...rowProps}
      className={`session-row ${personal ? 'recent-chat' : ''} ${active ? 'active' : ''} ${rowProps?.className ?? ''}`}
      onContextMenu={(event) => {
        event.preventDefault()
        onMenu(event.clientX, event.clientY)
      }}
    >
      {renaming ? (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void onRename()
          }}
        >
          <input
            autoFocus
            aria-label={t('Название чата')}
            value={title}
            onChange={(event) => onTitle(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') onCancelRename()
            }}
          />
        </form>
      ) : (
        <button
          className="session-link"
          title={sessionTitle(session.title, t('Новый чат'))}
          aria-current={active ? 'page' : undefined}
          onClick={onOpen}
          {...linkProps}
        >
          <MessageSquare size={14} />
          <span>{sessionTitle(session.title, t('Новый чат'))}</span>
          {busy && <SessionActivity />}
          {pinned && <Pin size={11} className="session-pin" aria-label={t('Закреплён')} />}
        </button>
      )}
      <button
        className="icon small row-delete"
        aria-label={t('Удалить чат')}
        title={t('Удалить чат')}
        onClick={onDelete}
      >
        <X size={13} />
      </button>
    </div>
  )
}
