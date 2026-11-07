const radius = 7
const length = 2 * Math.PI * radius

export function GoalRing({ ratio, size = 18 }: { ratio: number; size?: number }) {
  return (
    <svg className="goal-ring" width={size} height={size} viewBox="0 0 18 18" aria-hidden="true">
      <circle cx="9" cy="9" r={radius} className="goal-ring-track" />
      <circle
        cx="9"
        cy="9"
        r={radius}
        className="goal-ring-value"
        strokeDasharray={length}
        strokeDashoffset={length * (1 - Math.min(Math.max(ratio, 0), 1))}
      />
    </svg>
  )
}
