import { ArrowDown, ArrowUp, Search, X } from 'lucide-react'
import { useI18n } from '../../i18n'

export function ConversationSearch({
  query,
  change,
  count,
  active,
  move,
  close
}: {
  query: string
  change: (query: string) => void
  count: number
  active: number
  move: (delta: number) => void
  close: () => void
}) {
  const { t } = useI18n()
  return (
    <div className="conversation-search" role="search" aria-label={t('Поиск в чате')}>
      <Search size={16} />
      <input
        autoFocus
        value={query}
        aria-label={t('Поиск в чате')}
        placeholder={t('Найти в сообщениях…')}
        onChange={(event) => change(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            event.stopPropagation()
            close()
          }
          if (event.key === 'Enter') {
            event.preventDefault()
            move(event.shiftKey ? -1 : 1)
          }
        }}
      />
      <span role="status" aria-live="polite">
        {count ? `${active + 1} / ${count}` : query.trim() ? t('Нет совпадений') : '0 / 0'}
      </span>
      <button
        className="icon"
        disabled={!count}
        title={t('Предыдущее совпадение')}
        aria-label={t('Предыдущее совпадение')}
        onClick={() => move(-1)}
      >
        <ArrowUp size={15} />
      </button>
      <button
        className="icon"
        disabled={!count}
        title={t('Следующее совпадение')}
        aria-label={t('Следующее совпадение')}
        onClick={() => move(1)}
      >
        <ArrowDown size={15} />
      </button>
      <button className="icon" aria-label={t('Закрыть поиск')} onClick={close}>
        <X size={15} />
      </button>
    </div>
  )
}
