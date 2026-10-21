import type { Message } from '../../../../shared/contracts'
import { exportConversation, safeExportName } from '../../../../shared/workspaceTools'

export function downloadConversation(
  title: string,
  messages: Message[],
  t: (key: string) => string
) {
  const url = URL.createObjectURL(
    new Blob(
      [
        exportConversation(title, messages, {
          user: 'User',
          assistant: 'Prime',
          exported: t('Экспорт: {value0}')
        })
      ],
      {
        type: 'text/markdown;charset=utf-8'
      }
    )
  )
  const link = document.createElement('a')
  link.href = url
  link.download = safeExportName(title)
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

export function downloadConversationChunks(title: string, chunks: string[]) {
  const url = URL.createObjectURL(new Blob(chunks, { type: 'text/markdown;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = safeExportName(title)
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
