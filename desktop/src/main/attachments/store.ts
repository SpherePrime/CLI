import type { StoredAttachment } from '../../shared/attachments'

export function ensureAttachmentCapacity(
  store: Map<string, StoredAttachment>,
  bytes: number
): void {
  const retained = [...store.values()].reduce(
    (size, item) => size + Buffer.byteLength(item.content, 'base64'),
    0
  )
  if (retained + bytes > 200 * 1024 * 1024)
    throw new Error('Слишком много вложений. Удалите ненужные изображения.')
}

export function storeAttachments(
  store: Map<string, StoredAttachment>,
  attachments: { id: string; attachment: StoredAttachment }[]
): void {
  ensureAttachmentCapacity(
    store,
    attachments.reduce(
      (size, item) => size + Buffer.byteLength(item.attachment.content, 'base64'),
      0
    )
  )
  for (const { id, attachment } of attachments) store.set(id, attachment)
}
