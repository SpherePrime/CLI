import { useI18n } from '../../i18n'

const radius = 7
const circumference = 2 * Math.PI * radius

function compact(value: number) {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value >= 10_000_000 ? 0 : 1)}M`
  if (value >= 1000) return `${Math.round(value / 1000)}k`
  return String(value)
}

export function ContextMeter({ used, window }: { used: number; window: number }) {
  const { t } = useI18n()
  const ratio = Math.min(1, used / window)
  const percent = Math.round(ratio * 100)
  const level = ratio >= 0.85 ? 'high' : ratio >= 0.6 ? 'medium' : 'low'
  const label = t('Контекст: {used} из {total} ({percent}%)', {
    used: compact(used),
    total: compact(window),
    percent
  })
  return (
    <span className={`context-meter ${level}`} title={label} aria-label={label} role="img">
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
    </span>
  )
}
