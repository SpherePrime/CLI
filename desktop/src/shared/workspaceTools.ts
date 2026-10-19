import type { Message, Session } from './contracts'
import { textContent } from './messages'

export function findMessages(messages: Message[], query: string): string[] {
  const needle = query.trim().toLocaleLowerCase()
  if (!needle) return []
  return messages
    .filter(
      (message) =>
        ['user', 'assistant'].includes(message.role) &&
        textContent(message).toLocaleLowerCase().includes(needle)
    )
    .map((message) => message.id)
}

export function exportConversation(
  title: string,
  messages: Message[],
  labels: { user: string; assistant: string }
): string {
  const blocks = messages
    .filter(
      (message) => ['user', 'assistant'].includes(message.role) && textContent(message).trim()
    )
    .map(
      (message) =>
        `## ${message.role === 'user' ? labels.user : labels.assistant}\n\n${textContent(message).trim()}`
    )
  return [`# ${title.replace(/[\r\n]+/g, ' ').trim() || 'Chat'}`, ...blocks].join('\n\n') + '\n'
}

export function safeExportName(title: string): string {
  const name = title
    .trim()
    .replace(/[<>:"/\\|?*\u0000-\u001f]/g, '-')
    .replace(/[. ]+$/g, '')
    .slice(0, 80)
    .replace(/[. ]+$/g, '')
  return `${!name || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(name) ? 'chat' : name}.md`
}

export type SessionGroup = {
  id: 'pinned' | 'today' | 'yesterday' | 'week' | 'older'
  sessions: Session[]
}

export function groupSessions(
  sessions: Session[],
  pinned: string[],
  query = '',
  now = new Date()
): SessionGroup[] {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  const week = new Date(today)
  week.setDate(today.getDate() - 7)
  const pinSet = new Set(pinned)
  const groups: SessionGroup[] = ['pinned', 'today', 'yesterday', 'week', 'older'].map((id) => ({
    id: id as SessionGroup['id'],
    sessions: []
  }))
  const needle = query.trim().toLocaleLowerCase()
  const visible = sessions.filter(
    (session) => !session.parent_session_id && session.title.toLocaleLowerCase().includes(needle)
  )
  for (const session of [...visible].sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0))) {
    const timestamp = (session.updated_at || 0) * 1000
    const index = pinSet.has(session.id)
      ? 0
      : timestamp >= +today
        ? 1
        : timestamp >= +yesterday
          ? 2
          : timestamp >= +week
            ? 3
            : 4
    groups[index].sessions.push(session)
  }
  return groups.filter((group) => group.sessions.length)
}
