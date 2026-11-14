import type { PrimeAILimitWindow, PrimeAILimits } from '../../../../shared/contracts'
import type { TranslationValues } from '../../../../shared/localization'

export type LimitLevel = 'normal' | 'warn' | 'critical'
export type LimitTranslator = (source: string, values?: TranslationValues) => string

export function clampLimitPercent(value: number | undefined): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

export function getLimitLevel(percent: number | undefined): LimitLevel {
  const clamped = clampLimitPercent(percent)
  if (clamped >= 90) return 'critical'
  if (clamped >= 75) return 'warn'
  return 'normal'
}

export function isUnlimitedLimitWindow(window?: PrimeAILimitWindow): boolean {
  return Boolean(window && window.limitTokens === 0 && window.limitRequests === 0)
}

export function getHighestLimitPercent(limits: PrimeAILimits): number {
  const candidates: number[] = []
  if (limits.fiveHour && !isUnlimitedLimitWindow(limits.fiveHour)) {
    candidates.push(clampLimitPercent(limits.fiveHour.percent))
  }
  if (limits.weekly && !isUnlimitedLimitWindow(limits.weekly)) {
    candidates.push(clampLimitPercent(limits.weekly.percent))
  }
  return candidates.length ? Math.max(...candidates) : 0
}

export function getMaxLimitsLevel(limits: PrimeAILimits): LimitLevel {
  return getLimitLevel(getHighestLimitPercent(limits))
}

export function formatCompactLimitCount(value: number): string {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  if (safe >= 1_000_000) {
    const millions = safe / 1_000_000
    const digits = safe >= 10_000_000 || Number.isInteger(millions) ? 0 : 1
    return `${millions.toFixed(digits)}M`
  }
  if (safe >= 1000) return `${Math.round(safe / 1000)}k`
  return String(Math.round(safe))
}

export function formatWindowUsage(
  window: PrimeAILimitWindow,
  t: LimitTranslator,
  locale: string
): string {
  if (isUnlimitedLimitWindow(window)) return t('Без лимита')
  const parts: string[] = []
  if (window.limitTokens > 0) {
    parts.push(
      `${formatCompactLimitCount(window.usedTokens)} / ${formatCompactLimitCount(window.limitTokens)} ${t('ток.')}`
    )
  }
  if (window.limitRequests > 0) {
    parts.push(
      `${window.usedRequests.toLocaleString(locale)} / ${window.limitRequests.toLocaleString(locale)} ${t('запр.')}`
    )
  }
  return parts.join(' · ')
}

export function formatResetCountdown(
  window: PrimeAILimitWindow | undefined,
  t: LimitTranslator
): string {
  if (!window) return ''
  let seconds = Number.isFinite(window.resetInSeconds) ? Math.floor(window.resetInSeconds) : 0
  if (seconds <= 0 && window.resetAt) {
    const parsed = Date.parse(window.resetAt)
    if (Number.isFinite(parsed)) {
      seconds = Math.max(0, Math.floor((parsed - Date.now()) / 1000))
    }
  }
  if (seconds <= 0) return ''
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  let formatted = ''
  if (days > 0) {
    formatted = hours > 0 ? `${days}${t('д')} ${hours}${t('ч')}` : `${days}${t('д')}`
  } else if (hours > 0) {
    formatted = minutes > 0 ? `${hours}${t('ч')} ${minutes}${t('м')}` : `${hours}${t('ч')}`
  } else {
    formatted = `${Math.max(1, minutes)}${t('м')}`
  }
  return t('Сброс через {time}', { time: formatted })
}

export function formatUpdatedTimestamp(updatedAt: string | undefined, locale: string): string {
  if (!updatedAt) return ''
  const parsed = Date.parse(updatedAt)
  if (!Number.isFinite(parsed)) return ''
  return new Date(parsed).toLocaleTimeString(locale, {
    hour: '2-digit',
    minute: '2-digit'
  })
}
