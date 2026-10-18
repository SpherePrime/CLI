import { useEffect, useState } from 'react'

export function useElapsedSeconds(since: number | null) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (since === null) return
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [since])
  return since === null ? 0 : Math.max(0, Math.floor((now - since) / 1000))
}

export function clock(seconds: number) {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
}

export function duration(
  seconds: number,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  if (seconds < 60) return t('{seconds} с', { seconds: Math.max(1, seconds) })
  return t('{minutes} мин {seconds} с', {
    minutes: Math.floor(seconds / 60),
    seconds: seconds % 60
  })
}
