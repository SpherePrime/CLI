import { sessionTitle } from '../../../../shared/sessionTitle'
import { useEffect, useId, useRef, useState, type ElementType } from 'react'
import { ArrowUpRight, Command, MessageSquare, Search } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import type { Session } from '../../../../shared/contracts'

export type QuickAction = {
  id: string
  label: string
  icon: ElementType
  shortcut?: string
  disabled?: boolean
  run: () => void
}

export function CommandPalette({
  actions,
  sessions,
  selectSession,
  close
}: {
  actions: QuickAction[]
  sessions: Session[]
  selectSession: (session: Session) => void
  close: () => void
}) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)
  const input = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const listId = useId()
  const needle = query.trim().toLocaleLowerCase()
  const items = [
    ...actions.filter(
      (action) => !action.disabled && action.label.toLocaleLowerCase().includes(needle)
    ),
    ...sessions
      .filter(
        (session) =>
          !session.parent_session_id &&
          sessionTitle(session.title, t('Новый чат')).toLocaleLowerCase().includes(needle)
      )
      .sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0))
      .slice(0, 30)
      .map((session) => ({
        id: `session-${session.id}`,
        label: sessionTitle(session.title, t('Новый чат')),
        icon: MessageSquare,
        shortcut: t('Чат'),
        run: () => selectSession(session)
      }))
  ]
  const active = Math.min(selected, Math.max(0, items.length - 1))
  useEffect(() => {
    list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
  }, [active, query])
  function execute(index: number) {
    const item = items[index]
    if (!item) return
    close()
    item.run()
  }
  return (
    <Modal
      title={t('Быстрые действия')}
      close={close}
      initialFocus={input}
      className="command-palette"
    >
      <div className="command-search">
        <Search size={20} />
        <input
          ref={input}
          role="combobox"
          aria-label={t('Найти действие или чат')}
          aria-expanded="true"
          aria-controls={listId}
          aria-activedescendant={items.length ? `${listId}-${active}` : undefined}
          autoComplete="off"
          placeholder={t('Найти действие или чат…')}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value)
            setSelected(0)
          }}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
              event.preventDefault()
              setSelected(
                items.length
                  ? (active + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
                  : 0
              )
            }
            if (event.key === 'Enter') {
              event.preventDefault()
              execute(active)
            }
          }}
        />
        <kbd>Esc</kbd>
      </div>
      <div
        ref={list}
        id={listId}
        role="listbox"
        aria-label={t('Быстрые действия')}
        className="command-results"
      >
        {items.map((item, index) => (
          <button
            key={item.id}
            id={`${listId}-${index}`}
            role="option"
            aria-selected={index === active}
            tabIndex={-1}
            onPointerMove={() => setSelected(index)}
            onClick={() => execute(index)}
          >
            <span className="command-icon">
              <item.icon size={17} />
            </span>
            <span>{item.label}</span>
            {item.shortcut && <kbd>{item.shortcut}</kbd>}
            <ArrowUpRight size={14} className="command-enter" />
          </button>
        ))}
        {!items.length && (
          <div className="command-empty">
            <Search size={26} />
            <strong>{t('Ничего не найдено')}</strong>
            <span>{t('Попробуйте другое название')}</span>
          </div>
        )}
      </div>
      <footer className="command-footer">
        <Command size={13} />
        <span>{t('Действия и чаты в одном месте')}</span>
        <kbd>↑ ↓</kbd>
        <kbd>Enter</kbd>
      </footer>
    </Modal>
  )
}
