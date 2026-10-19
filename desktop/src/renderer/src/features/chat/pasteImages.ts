import type { ImageUpload } from '../../../../shared/attachments'

export function clipboardImages(clipboard: DataTransfer): File[] {
  const files = [...clipboard.files]
  for (const item of clipboard.items) {
    if (item.kind !== 'file') continue
    const file = item.getAsFile()
    if (
      file &&
      !files.some(
        (existing) =>
          existing.name === file.name && existing.size === file.size && existing.type === file.type
      )
    )
      files.push(file)
  }
  return files.filter(
    (file) => file.type.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(file.name)
  )
}

export async function imageUploads(files: File[]): Promise<ImageUpload[]> {
  if (files.some((file) => file.size > 20 * 1024 * 1024)) throw new Error('Вложение больше 20 MiB')
  return Promise.all(
    files.map(async (file) => ({
      name: file.name || 'screenshot.png',
      data: new Uint8Array(await file.arrayBuffer())
    }))
  )
}
