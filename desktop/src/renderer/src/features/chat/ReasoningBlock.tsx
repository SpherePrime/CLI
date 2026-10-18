import { ChevronDown, Sparkles } from 'lucide-react'
import { useI18n } from '../../i18n'
import { clock, duration, useElapsedSeconds } from './elapsed'

export function ReasoningBlock({
  thinking,
  startedAt,
  finishedAt,
  live
}: {
  thinking: string
  startedAt?: number
  finishedAt?: number
  live: boolean
}) {
  const { t } = useI18n()
  const elapsed = useElapsedSeconds(live && startedAt ? startedAt * 1000 : null)
  const spent = startedAt && finishedAt ? finishedAt - startedAt : 0
  const label = live
    ? `${t('Размышляет')}${startedAt ? ' · ' + clock(elapsed) : '…'}`
    : spent > 0
      ? t('Размышлял {time}', { time: duration(spent, t) })
      : t('Размышления')
  return (
    <details className={`reasoning ${live ? 'live' : ''}`}>
      <summary>
        <Sparkles size={14} />
        <span className="reasoning-label">{label}</span>
        <ChevronDown size={13} className="reasoning-chevron" />
      </summary>
      <div className="reasoning-text">{thinking}</div>
    </details>
  )
}
