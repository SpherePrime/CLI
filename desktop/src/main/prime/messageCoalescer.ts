import type { DesktopEvent } from '../../shared/contracts'

export const messageCoalesceMs = 75

function updatedMessageId(event: DesktopEvent): string | undefined {
  if (event.type !== 'message' || event.payload?.action !== 'updated') return undefined
  const id = event.payload?.data?.id
  return typeof id === 'string' && id ? id : undefined
}

export class MessageCoalescer {
  private pending = new Map<string, DesktopEvent>()
  private timer: ReturnType<typeof setTimeout> | undefined

  constructor(
    private readonly emit: (event: DesktopEvent) => void,
    private readonly delay = messageCoalesceMs
  ) {}

  push(event: DesktopEvent) {
    const id = updatedMessageId(event)
    if (!id) {
      this.flush()
      this.emit(event)
      return
    }
    this.pending.set(id, event)
    this.timer ??= setTimeout(() => this.flush(), this.delay)
  }

  flush() {
    if (this.timer) clearTimeout(this.timer)
    this.timer = undefined
    if (!this.pending.size) return
    const events = [...this.pending.values()]
    this.pending.clear()
    for (const event of events) this.emit(event)
  }

  dispose() {
    if (this.timer) clearTimeout(this.timer)
    this.timer = undefined
    this.pending.clear()
  }
}
