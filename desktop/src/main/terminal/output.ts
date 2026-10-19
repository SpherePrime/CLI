export class TerminalOutput {
  private chunks: Buffer[] = []
  private bytes = 0
  truncated = false

  constructor(private readonly limit = 1024 * 1024) {
    if (!Number.isInteger(limit) || limit < 4 || limit > 1024 * 1024)
      throw new Error('Invalid terminal output limit')
  }

  append(data: string) {
    const chunk = Buffer.from(data, 'utf8')
    this.chunks.push(chunk)
    this.bytes += chunk.length
    while (this.bytes > this.limit && this.chunks.length) {
      this.truncated = true
      const first = this.chunks[0]
      const excess = this.bytes - this.limit
      if (first.length <= excess) {
        this.chunks.shift()
        this.bytes -= first.length
      } else {
        let start = excess
        while (start < first.length && (first[start] & 0xc0) === 0x80) start += 1
        this.chunks[0] = first.subarray(start)
        this.bytes -= start
      }
    }
    if (this.chunks.length > 256) this.chunks = [Buffer.concat(this.chunks, this.bytes)]
  }

  read() {
    return Buffer.concat(this.chunks, this.bytes).toString('utf8')
  }
}
