import { useRef, useState } from 'react'
import type { PrimeAILimits } from '../../../../shared/contracts'
import { Popover } from '../../components/Popover'
import { useI18n } from '../../i18n'
import { PrimeLimitsDetails } from './PrimeLimitsDetails'
import {
  clampLimitPercent,
  getHighestLimitPercent,
  getMaxLimitsLevel,
  isUnlimitedLimitWindow
} from './primeLimitsFormat'

export function PrimeLimitsWidget({
  limits,
  onRefresh,
  refreshing = false
}: {
  limits: PrimeAILimits | null
  onRefresh: () => Promise<unknown> | void
  refreshing?: boolean
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [pendingRefresh, setPendingRefresh] = useState(false)
  const anchor = useRef<HTMLButtonElement>(null)

  if (!limits?.available) return null

  const hasFiveHour = Boolean(limits.fiveHour)
  const hasWeekly = Boolean(limits.weekly)
  if (!hasFiveHour && !hasWeekly) return null

  const level = getMaxLimitsLevel(limits)
  const fiveHourPct = limits.fiveHour
    ? isUnlimitedLimitWindow(limits.fiveHour)
      ? '∞'
      : `${Math.round(clampLimitPercent(limits.fiveHour.percent))}%`
    : ''
  const weeklyPct = limits.weekly
    ? isUnlimitedLimitWindow(limits.weekly)
      ? '∞'
      : `${Math.round(clampLimitPercent(limits.weekly.percent))}%`
    : ''
  const highestPct = Math.round(getHighestLimitPercent(limits))

  const summaryParts: string[] = []
  if (hasFiveHour) summaryParts.push(`${t('5ч')} ${fiveHourPct}`)
  if (hasWeekly) summaryParts.push(`${t('7д')} ${weeklyPct}`)
  const summaryLabel = t('PrimeAI Лимиты: {summary}', { summary: summaryParts.join(' · ') })

  async function handleRefresh() {
    setPendingRefresh(true)
    try {
      await onRefresh()
    } finally {
      setPendingRefresh(false)
    }
  }

  return (
    <>
      <button
        ref={anchor}
        type="button"
        className={`topbar-limits is-${level}${open ? ' is-open' : ''}`}
        title={summaryLabel}
        aria-label={summaryLabel}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className={`topbar-limits-dot is-${level}`} aria-hidden="true" />
        <span className="topbar-limits-pair" aria-hidden="true">
          {hasFiveHour && (
            <span className="topbar-limits-item">
              <span className="topbar-limits-prefix">{t('5ч')} </span>
              <span className="topbar-limits-pct">{fiveHourPct}</span>
            </span>
          )}
          {hasFiveHour && hasWeekly && (
            <>
              <span className="topbar-limits-sep-desktop"> · </span>
              <span className="topbar-limits-sep-mobile"> / </span>
            </>
          )}
          {hasWeekly && (
            <span className="topbar-limits-item topbar-limits-secondary">
              <span className="topbar-limits-prefix">{t('7д')} </span>
              <span className="topbar-limits-pct">{weeklyPct}</span>
            </span>
          )}
        </span>
        <span className="topbar-limits-compact-max" aria-hidden="true">
          {highestPct}%
        </span>
      </button>
      {open && (
        <Popover anchor={anchor} close={() => setOpen(false)} width={336} align="end" label={t('PrimeAI Лимиты')}>
          <PrimeLimitsDetails
            limits={limits}
            onRefresh={handleRefresh}
            refreshing={refreshing || pendingRefresh}
            variant="popover"
          />
        </Popover>
      )}
    </>
  )
}
