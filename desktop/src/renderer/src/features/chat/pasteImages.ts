import type { AttachmentUpload, ImageUpload } from '../../../../shared/attachments'

export function hasTransferFiles(data: DataTransfer | null | undefined): boolean {
  if (!data) return false
  if (data.files && data.files.length > 0) return true
  if (data.types && Array.from(data.types).includes('Files')) return true
  if (data.items) {
    for (const item of Array.from(data.items)) {
      if (item.kind === 'file') return true
    }
  }
  return false
}

export function clipboardFiles(data: DataTransfer | null | undefined): File[] {
  if (!data) return []
  const files = data.files ? [...data.files] : []
  if (data.items) {
    for (const item of Array.from(data.items)) {
      if (item.kind !== 'file') continue
      const file = item.getAsFile()
      if (
        file &&
        !files.some(
          (existing) =>
            existing.name === file.name &&
            existing.size === file.size &&
            existing.type === file.type
        )
      ) {
        files.push(file)
      }
    }
  }
  return files
}

export function clipboardImages(clipboard: DataTransfer): File[] {
  return clipboardFiles(clipboard).filter(
    (file) => file.type.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/i.test(file.name)
  )
}

export async function fileUploads(files: File[]): Promise<AttachmentUpload[]> {
  if (files.some((file) => file.size > 20 * 1024 * 1024)) throw new Error('Вложение больше 20 MiB')
  return Promise.all(
    files.map(async (file) => ({
      name: file.name || (file.type.startsWith('image/') ? 'screenshot.png' : 'attachment'),
      data: new Uint8Array(await file.arrayBuffer())
    }))
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
