import type { CSSProperties } from 'react'

export function ReasoningMeter({ level, total }: { level: number; total: number }) {
  return (
    <span className="reasoning-meter" aria-hidden="true">
      {Array.from({ length: total }, (_, index) => (
        <span
          key={index}
          className={index < level ? 'lit' : ''}
          style={{ '--bar': index, '--bars': total } as CSSProperties}
        />
      ))}
    </span>
  )
}
