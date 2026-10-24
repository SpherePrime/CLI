export function createAgentPolling<T>({
  load,
  receive,
  failed,
  interval,
  hidden
}: {
  load: () => Promise<T>
  receive: (value: T) => void
  failed: (error: unknown) => void
  interval: () => number
  hidden: () => boolean
}) {
  let disposed = false
  let pending = false
  let timer: ReturnType<typeof setTimeout> | undefined
  const refresh = () => {
    if (disposed || pending) return
    clearTimeout(timer)
    if (hidden()) {
      timer = setTimeout(refresh, interval())
      return
    }
    pending = true
    void load()
      .then((value) => {
        if (!disposed) receive(value)
      })
      .catch((error: unknown) => {
        if (!disposed) failed(error)
      })
      .finally(() => {
        pending = false
        if (!disposed) timer = setTimeout(refresh, interval())
      })
  }
  return {
    refresh,
    dispose: () => {
      disposed = true
      clearTimeout(timer)
    }
  }
}
