const lockMessage = /data directory already in use/i

export const isProjectLocked = (error: unknown) => lockMessage.test(String((error as Error)?.message ?? error))

type Options<T> = {
  open(): Promise<T>
  onWait(): void
  onDone(): void
  signal: AbortSignal
  interval?: number
  timeout?: number
}
export async function openWhenUnlocked<T>({
  open,
  onWait,
  onDone,
  signal,
  interval = 2000,
  timeout = 15 * 60 * 1000
}: Options<T>): Promise<T> {
  const started = Date.now()
  let waiting = false
  try {
    while (true) {
      try {
        return await open()
      } catch (error) {
        if (!isProjectLocked(error)) throw error
        if (signal.aborted || Date.now() - started > timeout) throw error
        if (!waiting) {
          waiting = true
          onWait()
        }
        await new Promise<void>((resolve) => {
          const timer = setTimeout(resolve, interval)
          signal.addEventListener('abort', () => {
            clearTimeout(timer)
            resolve()
          }, { once: true })
        })
        if (signal.aborted) throw Object.assign(new Error('project.waitCancelled'), { cancelled: true })
      }
    }
  } finally {
    if (waiting) onDone()
  }
}
