import { forwardRef } from 'react'
import { useI18n } from '../../i18n'

const radius = 7
const circumference = 2 * Math.PI * radius

export function compactTokens(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}M`
  if (value >= 1000) return `${Math.round(value / 1000)}k`
  return String(value)
}

export const ContextMeter = forwardRef<
  HTMLButtonElement,
  { used: number; window: number; expanded: boolean; onClick: () => void }
>(function ContextMeter({ used, window, expanded, onClick }, ref) {
  const { t } = useI18n()
  const ratio = Math.min(1, used / window)
  const percent = Math.round(ratio * 100)
  const level = ratio >= 0.85 ? 'high' : ratio >= 0.6 ? 'medium' : 'low'
  const label = t('Контекст: {used} из {total} ({percent}%)', {
    used: compactTokens(used),
    total: compactTokens(window),
    percent
  })
  return (
    <button
      ref={ref}
      className={`context-meter ${level}`}
      title={label}
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={expanded}
      onClick={onClick}
    >
      <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true">
        <circle cx="9" cy="9" r={radius} className="context-meter-track" />
        <circle
          cx="9"
          cy="9"
          r={radius}
          className="context-meter-value"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - ratio)}
        />
      </svg>
    </button>
  )
})
