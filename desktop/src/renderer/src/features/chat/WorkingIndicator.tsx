import { useEffect, useState } from 'react'
import { useI18n } from '../../i18n'
import { clock, useElapsedSeconds } from './elapsed'

export function WorkingIndicator({ busy, waiting }: { busy: boolean; waiting: boolean }) {
  const { t } = useI18n()
  const [busySince, setBusySince] = useState<number | null>(null)
  useEffect(() => setBusySince(busy ? Date.now() : null), [busy])
  const working = useElapsedSeconds(busySince)
  if (!busy && !waiting) return null
  return (
    <div className="working" role="status">
      <span className="working-orb" aria-hidden="true">
        <span />
        <span />
        <span />
      </span>
      <span className="working-label">
        {waiting ? t('Ожидается подтверждение') : t('Prime работает')}
      </span>
      {busy && !waiting && <span className="working-time">{clock(working)}</span>}
    </div>
  )
}
