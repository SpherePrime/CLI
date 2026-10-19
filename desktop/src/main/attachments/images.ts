import { randomUUID } from 'node:crypto'
import type { Attachment, ImageUpload, StoredAttachment } from '../../shared/attachments'

function mimeType(bytes: Buffer): string {
  if (bytes.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])))
    return 'image/png'
  if (bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255) return 'image/jpeg'
  if (/^GIF8[79]a$/.test(bytes.subarray(0, 6).toString('ascii'))) return 'image/gif'
  if (
    bytes.subarray(0, 4).toString('ascii') === 'RIFF' &&
    bytes.subarray(8, 12).toString('ascii') === 'WEBP'
  )
    return 'image/webp'
  throw new Error('Не удалось прочитать изображение')
}

export function importImages(
  store: Map<string, StoredAttachment>,
  input: unknown,
  preview: (bytes: Buffer) => string
): Attachment[] {
  if (!Array.isArray(input) || !input.length || input.length > 10)
    throw new Error('За один раз можно вставить до 10 изображений')
  let total = 0
  const prepared = input.map((item: ImageUpload) => {
    if (!item || typeof item.name !== 'string' || !(item.data instanceof Uint8Array))
      throw new Error('Некорректное вложение')
    const bytes = Buffer.from(item.data.buffer, item.data.byteOffset, item.data.byteLength)
    if (!bytes.length || bytes.length > 20 * 1024 * 1024) throw new Error('Вложение больше 20 MiB')
    total += bytes.length
    const type = mimeType(bytes)
    const name =
      item.name
        .replace(/[\\/\x00-\x1f]/g, '_')
        .slice(0, 200)
        .trim() || 'screenshot.png'
    return { bytes, type, name, id: randomUUID() }
  })
  const retained = [...store.values()].reduce((size, item) => size + item.content.length * 0.75, 0)
  if (retained + total > 200 * 1024 * 1024)
    throw new Error('Слишком много вложений. Удалите ненужные изображения.')
  const ready = prepared.map((item) => ({ ...item, preview: preview(item.bytes) }))
  for (const item of ready)
    store.set(item.id, {
      file_path: item.name,
      file_name: item.name,
      mime_type: item.type,
      content: item.bytes.toString('base64')
    })
  return ready.map((item) => ({
    id: item.id,
    name: item.name,
    size: item.bytes.length,
    mimeType: item.type,
    preview: item.preview
  }))
}
