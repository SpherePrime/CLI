import { createReadStream } from 'node:fs'
import { createHash } from 'node:crypto'

export async function checksum(path: string): Promise<string> {
  const hash = createHash('sha256')
  for await (const bytes of createReadStream(path)) hash.update(bytes)
  return hash.digest('hex')
}
