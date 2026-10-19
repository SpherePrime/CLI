import type { Message } from '../../../../shared/contracts'
import { exportConversation, safeExportName } from '../../../../shared/workspaceTools'

export function downloadConversation(title: string, messages: Message[], user: string) {
  const url = URL.createObjectURL(
    new Blob([exportConversation(title, messages, { user, assistant: 'Prime' })], {
      type: 'text/markdown;charset=utf-8'
    })
  )
  const link = document.createElement('a')
  link.href = url
  link.download = safeExportName(title)
  document.body.append(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
