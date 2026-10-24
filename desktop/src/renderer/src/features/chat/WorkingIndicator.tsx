import { useI18n } from '../../i18n'
import { clock, useElapsedSeconds } from './elapsed'

export function WorkingIndicator({
  busy,
  waiting,
  retrying = false,
  startedAt
}: {
  busy: boolean
  waiting: boolean
  retrying?: boolean
  startedAt: number | null
}) {
  const { t } = useI18n()
  const working = useElapsedSeconds(busy ? startedAt : null)
  if (!busy && !waiting) return null
  return (
    <div className="working" role="status">
      <span className="working-orb" aria-hidden="true">
        <span />
        <span />
        <span />
      </span>
      <span className="working-label">
        {waiting
          ? t('Ожидается подтверждение')
          : retrying
            ? t('Ответ прервался у провайдера — повторяю запрос')
            : t('Prime работает')}
      </span>
      {busy && !waiting && startedAt !== null && (
        <span className="working-time">{clock(working)}</span>
      )}
    </div>
  )
}
