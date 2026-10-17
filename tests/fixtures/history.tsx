import '../../src/renderer/src/styles/global.css'
import React, { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { MessageList } from '../../src/renderer/src/features/chat/MessageList'
import type { Message } from '../../src/shared/contracts'
function History() {
  const [messages, setMessages] = useState<Message[]>(
    Array.from(
      { length: 1000 },
      (_, index) =>
        ({
          id: `fixture-${index}`,
          role: index % 2 ? 'assistant' : 'user',
          parts: [
            {
              type: 'text',
              data: {
                text: `Message ${index}\n\n` + 'Variable row content. '.repeat((index % 9) + 1)
              }
            }
          ]
        }) as Message
    )
  )
  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <button
        onClick={() =>
          setMessages((items) => [
            ...items,
            {
              id: 'new',
              role: 'assistant',
              parts: [{ type: 'text', data: { text: 'Newest streamed answer' } }]
            } as Message
          ])
        }
      >
        Append
      </button>
      <MessageList messages={messages} busy={false} />
    </div>
  )
}
createRoot(document.getElementById('root')!).render(<History />)
