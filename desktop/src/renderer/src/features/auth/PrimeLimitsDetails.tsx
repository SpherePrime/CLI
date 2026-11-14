import { AlertCircle, RefreshCw } from 'lucide-react'
import type { PrimeAILimitWindow, PrimeAILimits } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import {
  clampLimitPercent,
  formatResetCountdown,
  formatUpdatedTimestamp,
  getLimitLevel,
  isUnlimitedLimitWindow
} from './primeLimitsFormat'

function LimitWindowRow({
  label,
  windowData
}: {
  label: string
  windowData: PrimeAILimitWindow
}) {
  const { t } = useI18n()
  const unlimited = isUnlimitedLimitWindow(windowData)
  const percent = unlimited ? 0 : clampLimitPercent(windowData.percent)
  const roundedPercent = Math.round(percent)
  const level = unlimited ? 'normal' : getLimitLevel(percent)
  const resetText = formatResetCountdown(windowData, t)

  return (
    <div className="prime-limits-row">
      <div className="prime-limits-row-top">
        <span className="prime-limits-row-label">{label}</span>
        <span
          className={`prime-limits-row-pct is-${level}${unlimited ? ' is-unlimited' : ''}`}
        >
          {unlimited ? t('Без лимита') : `${roundedPercent}%`}
        </span>
      </div>
      <div
        className="prime-limits-track"
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={roundedPercent}
      >
        <span
          className={`prime-limits-fill is-${level}`}
          style={{ width: `${percent}%` }}
        />
      </div>
      {resetText && (
        <div className="prime-limits-row-meta">
          <span className="prime-limits-reset">{resetText}</span>
        </div>
      )}
    </div>
  )
}

export function PrimeLimitsDetails({
  limits,
  onRefresh,
  refreshing = false,
  variant = 'popover'
}: {
  limits: PrimeAILimits | null
  onRefresh?: () => Promise<unknown> | void
  refreshing?: boolean
  variant?: 'popover' | 'card'
}) {
  const { t, locale } = useI18n()
  const updatedTime = formatUpdatedTimestamp(limits?.updatedAt, locale)

  return (
    <div className={`prime-limits-panel is-${variant}`}>
      <div className="prime-limits-head">
        <div className="prime-limits-title-group">
          <strong>{t('PrimeAI Лимиты')}</strong>
          {limits?.available && limits.planName && (
            <span
              className="prime-limits-plan"
              title={t('Тариф: {plan}', { plan: limits.planName })}
            >
              {limits.planName}
            </span>
          )}
        </div>
        {onRefresh && (
          <button
            type="button"
            className="prime-limits-refresh"
            disabled={refreshing}
            aria-busy={refreshing}
            onClick={() => void onRefresh()}
          >
            <RefreshCw size={12} className={refreshing ? 'spin' : undefined} aria-hidden="true" />
            <span>{refreshing ? t('Обновляем…') : t('Обновить')}</span>
          </button>
        )}
      </div>

      {!limits ? (
        <p className="prime-limits-empty" role="status">
          {t('Загрузка лимитов…')}
        </p>
      ) : limits.available ? (
        <>
          <div className="prime-limits-rows">
            {limits.fiveHour && (
              <LimitWindowRow label={t('5-часовой лимит')} windowData={limits.fiveHour} />
            )}
            {limits.weekly && (
              <LimitWindowRow label={t('Недельный лимит')} windowData={limits.weekly} />
            )}
          </div>
          {limits.error && (
            <p className="prime-limits-error" role="alert">
              <AlertCircle size={13} aria-hidden="true" />
              <span>{limits.error}</span>
            </p>
          )}
          {updatedTime && (
            <div className="prime-limits-footer">
              <span className="prime-limits-updated">
                {t('Обновлено в {time}', { time: updatedTime })}
              </span>
            </div>
          )}
        </>
      ) : limits.error ? (
        <p className="prime-limits-error" role="alert">
          <AlertCircle size={13} aria-hidden="true" />
          <span>{limits.error}</span>
        </p>
      ) : (
        <p className="prime-limits-empty" role="status">
          {t('Лимиты PrimeAI недоступны')}
        </p>
      )}
    </div>
  )
}
