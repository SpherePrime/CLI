import type { Message } from './contracts'
export function applyMessage(messages: Message[], message: Message): Message[] {
  const index = messages.findIndex((item) => item.id === message.id)
  if (index < 0) return [...messages, message]
  return messages.map((item, position) => (position === index ? message : item))
}
export function visibleMessageText(text: string, role: string): string {
  if (role !== 'user') return text
  return text.replace(
    /\n\n<working-context>\n(?=(?:Task goal:|Constraints:|Acceptance criteria:|Source:))[\s\S]*\n<\/working-context>$/,
    ''
  )
}
export function textContent(message: Message): string {
  return (message.parts || [])
    .filter((part) => part.type === 'text' && !part.data.hidden)
    .map((part) => visibleMessageText(part.data.text || '', message.role))
    .join('\n')
}
