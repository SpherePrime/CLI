import type { Message } from './contracts'
export function applyMessage(messages: Message[], message: Message): Message[] {
  const index = messages.findIndex((item) => item.id === message.id)
  if (index < 0) return [...messages, message]
  return messages.map((item, position) => (position === index ? message : item))
}
export function textContent(message: Message): string {
  return (message.parts || [])
    .filter((part) => part.type === 'text' && !part.data.hidden)
    .map((part) => part.data.text || '')
    .join('\n')
}
