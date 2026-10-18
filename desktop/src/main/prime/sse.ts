import { StringDecoder } from 'node:string_decoder'
export class SseDecoder {
  private decoder = new StringDecoder('utf8')
  private buffer = ''
  push(chunk: Buffer): string[] {
    this.buffer += this.decoder.write(chunk)
    const output: string[] = []
    let boundary: RegExpExecArray | null
    while ((boundary = /\r?\n\r?\n/.exec(this.buffer))) {
      const block = this.buffer.slice(0, boundary.index)
      this.buffer = this.buffer.slice(boundary.index + boundary[0].length)
      const data = block
        .split(/\r?\n/)
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).replace(/^ /, ''))
        .join('\n')
      if (data) output.push(data)
    }
    if (this.buffer.length > 32 * 1024 * 1024) throw new Error('Слишком большое событие Prime')
    return output
  }
}
