export function PrimeLogo({ className = '' }: { className?: string }) {
  return (
    <svg
      className={`prime-logo ${className}`}
      viewBox="0 0 36 36"
      aria-label="SpherePrime"
      role="img"
    >
      <g fill="currentColor" transform="translate(8 2) skewX(-20)">
        <rect y="7" width="7" height="22" rx="2" />
        <rect x="11" width="7" height="32" rx="2" />
        <rect x="22" y="7" width="7" height="22" rx="2" />
      </g>
    </svg>
  )
}
