import { useId, type CSSProperties } from 'react'

export function RangeField({
  label,
  value,
  min,
  max,
  step,
  formatValue,
  onChange,
  disabled = false
}: {
  label: string
  value: number
  min: number
  max: number
  step: number
  formatValue: (value: number) => string
  onChange: (value: number) => void
  disabled?: boolean
}) {
  const id = useId()
  const progress = Math.max(0, Math.min(100, ((value - min) / (max - min)) * 100))
  return (
    <div className={`range-field${disabled ? ' disabled' : ''}`}>
      <div className="range-field-heading">
        <label htmlFor={id}>{label}</label>
        <output htmlFor={id}>{formatValue(value)}</output>
      </div>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        aria-valuetext={formatValue(value)}
        style={{ '--range-progress': `${progress}%` } as CSSProperties}
        onChange={(event) => onChange(Number(event.target.value))}
      />
      <div className="range-field-limits" aria-hidden="true">
        <span>{formatValue(min)}</span>
        <span>{formatValue(max)}</span>
      </div>
    </div>
  )
}
