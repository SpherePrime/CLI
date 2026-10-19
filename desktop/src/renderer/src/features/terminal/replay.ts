import type { TerminalDataEvent } from '../../../../shared/terminal'

export class TerminalReplay {
  private ready = false
  private sequence = -1
  private buffered: (TerminalDataEvent & { bytes: number })[] = []
  private bufferedBytes = 0

  receive(event: TerminalDataEvent): string {
    if (!this.ready) {
      const encoded = new TextEncoder().encode(event.data)
      let start = Math.max(0, encoded.byteLength - 1024 * 1024)
      while (start < encoded.length && (encoded[start] & 0xc0) === 0x80) start += 1
      const bytes = encoded.byteLength - start
      const data = start ? new TextDecoder().decode(encoded.subarray(start)) : event.data
      this.buffered.push({ ...event, data, bytes })
      this.bufferedBytes += bytes
      while (this.bufferedBytes > 1024 * 1024) this.bufferedBytes -= this.buffered.shift()!.bytes
      return ''
    }
    if (event.sequence <= this.sequence) return ''
    this.sequence = event.sequence
    return event.data
  }

  restore(snapshot: { output: string; sequence: number }): string {
    this.sequence = snapshot.sequence
    this.ready = true
    const pending = this.buffered.sort((left, right) => left.sequence - right.sequence)
    this.buffered = []
    this.bufferedBytes = 0
    return snapshot.output + pending.map((event) => this.receive(event)).join('')
  }
}
