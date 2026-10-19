export type ImageUpload = { name: string; data: Uint8Array }
export type Attachment = {
  id: string
  name: string
  size: number
  mimeType: string
  preview?: string
}
export type StoredAttachment = {
  file_path: string
  file_name: string
  mime_type: string
  content: string
}
