import { useEffect, useState } from 'react'
import { ArrowUp, ListOrdered, LoaderCircle, Paperclip, Trash2 } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Workspace } from '../../app/useWorkspace'

export function QueuePanel({ workspace: w }: { workspace: Workspace }) {
  const { t, locale } = useI18n()
  const [pending, setPending] = useState('')
  const [steering, setSteering] = useState<string[]>([])
  const [error, setError] = useState('')
  useEffect(() => {
    setSteering((ids) => ids.filter((id) => w.queue.some((item) => item.id === id)))
  }, [w.queue])
  if (!w.queue.length) return null
  async function act(id: string) {
    if (pending || steering.includes(id) || !w.session) return
    const projectId = w.project!.id
    const sessionId = w.session.id
    setPending(id)
    setError('')
    try {
      await window.prime.call(projectId, id === 'clear' ? 'clearQueue' : 'runQueuedNow', {
        sessionId,
        id
      })
      if (id !== 'clear') setSteering((ids) => [...ids, id])
      await w.refreshQueue()
    } catch (error) {
      setError(error instanceof Error ? error.message : String(error))
      await w.refreshQueue()
    } finally {
      setPending('')
    }
  }
  return (
    <section className="prompt-queue" aria-label={t('Очередь сообщений')}>
      <header>
        <ListOrdered size={15} />
        <strong>{t('Следующие сообщения')}</strong>
        <span className="queue-count">{w.queue.length.toLocaleString(locale)}</span>
        <button
          className="icon small"
          title={t('Очистить очередь')}
          aria-label={t('Очистить очередь')}
          disabled={!!pending}
          onClick={() => void act('clear')}
        >
          <Trash2 size={14} />
        </button>
      </header>
      <ol>
        {w.queue.map((item, index) => (
          <li key={item.id}>
            <span className="queue-index">{(index + 1).toLocaleString(locale)}</span>
            <div className="queue-prompt">
              <p title={item.prompt}>{item.prompt}</p>
              {item.attachments > 0 && (
                <small>
                  <Paperclip size={12} /> {t('Вложения: {count}', { count: item.attachments })}
                </small>
              )}
            </div>
            <button
              className="queue-send-now"
              disabled={!!pending || steering.includes(item.id) || w.project?.connection !== 'ready'}
              title={t('Добавить к текущей задаче на следующем шаге')}
              onClick={() => void act(item.id)}
            >
              {pending === item.id || steering.includes(item.id) ? (
                <LoaderCircle size={14} className="spin" />
              ) : (
                <ArrowUp size={14} />
              )}
              <span>{steering.includes(item.id) ? t('Отправляется') : t('Отправить сейчас')}</span>
            </button>
          </li>
        ))}
      </ol>
      {error && (
        <p className="queue-error" role="alert">
          {error}
        </p>
      )}
    </section>
  )
}
