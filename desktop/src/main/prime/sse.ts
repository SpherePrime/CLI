import { StringDecoder } from 'node:string_decoder'
export class SseDecoder {
  private decoder = new StringDecoder('utf8')
  private buffer = ''
  private scanned = 0
  push(chunk: Buffer): string[] {
    this.buffer += this.decoder.write(chunk)
    const output: string[] = []
    const boundary = /\r?\n\r?\n/g
    boundary.lastIndex = Math.max(0, this.scanned - 3)
    let start = 0
    let match: RegExpExecArray | null
    while ((match = boundary.exec(this.buffer))) {
      const block = this.buffer.slice(start, match.index)
      start = match.index + match[0].length
      const data = block
        .split(/\r?\n/)
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).replace(/^ /, ''))
        .join('\n')
      if (data) output.push(data)
    }
    if (start) this.buffer = this.buffer.slice(start)
    this.scanned = this.buffer.length
    if (this.buffer.length > 32 * 1024 * 1024) throw new Error('Слишком большое событие Prime')
    return output
  }
}
