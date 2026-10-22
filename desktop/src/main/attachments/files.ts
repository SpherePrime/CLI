import { randomUUID } from 'node:crypto'
import { readFile, stat } from 'node:fs/promises'
import { basename, extname } from 'node:path'
import type { StoredAttachment } from '../../shared/attachments'
import { ensureAttachmentCapacity, storeAttachments } from './store'

const mimeTypes: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.pdf': 'application/pdf',
  '.gif': 'image/gif'
}

export async function importFiles(store: Map<string, StoredAttachment>, paths: string[]) {
  const output: { id: string; name: string; size: number }[] = []
  const prepared: { id: string; attachment: StoredAttachment }[] = []
  let total = 0
  for (const path of paths) {
    const info = await stat(path)
    if (info.size > 20 * 1024 * 1024) throw new Error('Вложение больше 20 MiB')
    ensureAttachmentCapacity(store, total + info.size)
    const bytes = await readFile(path)
    if (bytes.length > 20 * 1024 * 1024) throw new Error('Вложение больше 20 MiB')
    total += bytes.length
    ensureAttachmentCapacity(store, total)
    const id = randomUUID()
    prepared.push({
      id,
      attachment: {
        file_path: path,
        file_name: basename(path),
        mime_type: mimeTypes[extname(path).toLowerCase()] || 'text/plain',
        content: bytes.toString('base64')
      }
    })
    output.push({ id, name: basename(path), size: bytes.length })
  }
  storeAttachments(store, prepared)
  return output
}
