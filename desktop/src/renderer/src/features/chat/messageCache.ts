import type { Message } from '../../../../shared/contracts'

function retainedSize(value: unknown): number {
  if (typeof value === 'string') return value.length * 2
  if (value === null || value === undefined) return 0
  if (typeof value !== 'object') return 8
  if (Array.isArray(value)) return 40 + value.reduce((bytes, item) => bytes + retainedSize(item), 0)
  return Object.entries(value).reduce(
    (bytes, [key, item]) => bytes + key.length * 2 + retainedSize(item),
    64
  )
}

export class MessageCache {
  private readonly entries = new Map<string, { messages: Message[]; bytes: number }>()
  private readonly messageSizes = new WeakMap<Message, number>()
  private retained = 0

  constructor(
    private readonly maxSessions = 8,
    private readonly maxBytes = 32 * 1024 * 1024
  ) {}

  get size() {
    return this.entries.size
  }
  get bytes() {
    return this.retained
  }

  get(id: string): Message[] | undefined {
    const entry = this.entries.get(id)
    if (!entry) return undefined
    this.entries.delete(id)
    this.entries.set(id, entry)
    return entry.messages
  }

  set(id: string, messages: Message[]) {
    const previous = this.entries.get(id)
    if (previous?.messages === messages) {
      this.get(id)
      return
    }
    this.delete(id)
    let bytes = 0
    for (const message of messages) {
      let size = this.messageSizes.get(message)
      if (size === undefined) {
        size = retainedSize(message)
        this.messageSizes.set(message, size)
      }
      bytes += size
    }
    if (bytes > this.maxBytes || this.maxSessions < 1) return
    this.entries.set(id, { messages, bytes })
    this.retained += bytes
    while (this.entries.size > this.maxSessions || this.retained > this.maxBytes)
      this.delete(this.entries.keys().next().value!)
  }

  delete(id: string) {
    const entry = this.entries.get(id)
    if (!entry) return
    this.retained -= entry.bytes
    this.entries.delete(id)
  }
}
