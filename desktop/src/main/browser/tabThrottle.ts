import type { WebContents } from 'electron'

export const idleTabMs = 60_000
export const idleTabSweepMs = 15_000

type Usage = { usedAt: number; throttled: boolean }

export class TabThrottle {
  private usage = new Map<WebContents, Usage>()
  private timer: ReturnType<typeof setInterval> | undefined

  constructor(
    private readonly visible: (contents: WebContents) => boolean,
    private readonly idleMs = idleTabMs,
    private readonly sweepMs = idleTabSweepMs
  ) {}

  touch(contents: WebContents) {
    if (contents.isDestroyed()) return
    const usage = this.usage.get(contents)
    if (usage?.throttled) contents.setBackgroundThrottling?.(false)
    this.usage.set(contents, { usedAt: Date.now(), throttled: false })
    if (!this.timer) {
      this.timer = setInterval(() => this.sweep(), this.sweepMs)
      this.timer.unref?.()
    }
  }

  forget(contents: WebContents) {
    this.usage.delete(contents)
    if (!this.usage.size) this.stop()
  }

  sweep() {
    const now = Date.now()
    for (const [contents, usage] of this.usage) {
      if (contents.isDestroyed()) {
        this.usage.delete(contents)
        continue
      }
      if (usage.throttled || this.visible(contents)) continue
      if (now - usage.usedAt < this.idleMs) continue
      contents.setBackgroundThrottling?.(true)
      usage.throttled = true
    }
    if (!this.usage.size) this.stop()
  }

  dispose() {
    this.stop()
    this.usage.clear()
  }

  private stop() {
    if (this.timer) clearInterval(this.timer)
    this.timer = undefined
  }
}
