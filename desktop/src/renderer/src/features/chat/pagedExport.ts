import type { MessagePage, Operation } from '../../../../shared/contracts'
import { exportConversation } from '../../../../shared/workspaceTools'

export async function conversationExportChunks(
  projectId: string,
  sessionId: string,
  title: string,
  t: (key: string) => string,
  call: (projectId: string, operation: Operation, args: any) => Promise<any> = window.prime.call
): Promise<string[]> {
  const separator = '\n\n---\n\n'
  const blocks: string[] = []
  let before: string | undefined
  for (;;) {
    const page: MessagePage = await call(projectId, 'messagePage', {
      sessionId,
      limit: 100,
      before,
      textOnly: true
    })
    const formatted = exportConversation(title, page.messages, { user: 'User', assistant: 'Prime' })
    const body = formatted.indexOf(separator)
    if (body >= 0) blocks.unshift(formatted.slice(body + separator.length).trimEnd())
    if (!page.has_before || !page.before || page.before === before) break
    before = page.before
  }
  const header = exportConversation(title, [], {
    user: 'User',
    assistant: 'Prime',
    exported: t('Экспорт: {value0}')
  }).trimEnd()
  return [header, ...blocks.flatMap((block) => [separator, block]), '\n']
}
