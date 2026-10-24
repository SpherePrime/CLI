import { useEffect, useRef, useState } from 'react'
import { createAgentPolling } from './agentPolling'

export function useAgentPolling<T>(
  load: () => Promise<T>,
  interval: number | ((value: T) => number),
  initial: T
) {
  const [value, setValue] = useState(initial)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const intervalRef = useRef(3000)
  intervalRef.current = typeof interval === 'function' ? interval(value) : interval
  const refresh = useRef<() => void>(() => {})
  useEffect(() => {
    const polling = createAgentPolling({
      load,
      receive: (next) => {
        setValue(next)
        setLoading(false)
        setError('')
      },
      failed: (reason) => {
        setError(reason instanceof Error ? reason.message : String(reason))
        setLoading(false)
      },
      interval: () => intervalRef.current,
      hidden: () => document.hidden
    })
    refresh.current = polling.refresh
    const visible = () => {
      if (!document.hidden) polling.refresh()
    }
    document.addEventListener('visibilitychange', visible)
    polling.refresh()
    return () => {
      polling.dispose()
      refresh.current = () => {}
      document.removeEventListener('visibilitychange', visible)
    }
  }, [load])
  return { value, loading, error, refresh: () => refresh.current() }
}
