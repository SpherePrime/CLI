export class PickedAttachments {
  private current = new Set<string>()
  private sending = new Set<string>()
  private disposed = false

  constructor(private readonly release: (ids: string[]) => void) {}

  replace(ids: string[]): boolean {
    const previous = this.current
    this.current = new Set(this.disposed ? [] : ids)
    this.releaseUnused([...previous, ...(this.disposed ? ids : [])])
    return !this.disposed
  }

  beginSend(ids: string[]): void {
    for (const id of ids) this.sending.add(id)
  }

  endSend(ids: string[]): void {
    for (const id of ids) this.sending.delete(id)
    this.releaseUnused(ids)
  }

  dispose(): void {
    this.disposed = true
    this.replace([])
  }

  private releaseUnused(ids: string[]): void {
    const unused = [...new Set(ids)].filter((id) => !this.current.has(id) && !this.sending.has(id))
    if (unused.length) this.release(unused)
  }
}
