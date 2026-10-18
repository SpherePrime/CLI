import '../../src/renderer/src/styles/global.css'
import '../../src/renderer/src/styles/brand.css'
import React, { useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { MessageList } from '../../src/renderer/src/features/chat/MessageList'
import type { Message } from '../../src/shared/contracts'
function History() {
  const [streaming, setStreaming] = useState(false)
  const [ticks, setTicks] = useState(0)
  useEffect(() => {
    if (!streaming) return
    const timer = setInterval(() => {
      setTicks((value) => value + 1)
      setMessages((items) =>
        items.map((item, index) =>
          index === items.length - 1
            ? {
                ...item,
                parts: [{ type: 'text', data: { text: item.parts[0].data.text + ' streaming' } }]
              }
            : item
        )
      )
    }, 40)
    return () => clearInterval(timer)
  }, [streaming])
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
        onClick={() => {
          setTicks(0)
          setStreaming(!streaming)
        }}
      >
        {streaming ? 'Stop stream' : 'Start stream'}
      </button>
      <span data-testid="stream-ticks">{ticks}</span>
      <button
        onClick={() =>
          setMessages([
            {
              id: 'tool-user',
              session_id: 'fixture',
              role: 'user',
              parts: [{ type: 'text', data: { text: 'Проверь команду и покажи результат' } }]
            },
            {
              id: 'tool-call',
              session_id: 'fixture',
              role: 'assistant',
              parts: [
                {
                  type: 'tool_call',
                  data: {
                    id: 'bash-call',
                    name: 'bash',
                    finished: true,
                    input: JSON.stringify({
                      description: 'Вывести приветствие',
                      command: 'printf "Hello, Prime!\\n"'
                    })
                  }
                }
              ]
            }
          ])
        }
      >
        Tools fixture
      </button>
      <button
        onClick={() =>
          setMessages((items) => [
            ...items,
            {
              id: 'tool-output',
              session_id: 'fixture',
              role: 'tool',
              parts: [
                {
                  type: 'tool_result',
                  data: {
                    tool_call_id: 'bash-call',
                    name: 'bash',
                    content: 'Hello, Prime!\nProcess finished successfully.',
                    is_error: false
                  }
                },
                { type: 'finish', data: { reason: 'stop' } }
              ]
            }
          ])
        }
      >
        Finish tool
      </button>
      <button
        onClick={() =>
          setMessages([
            {
              id: 'reasoning',
              session_id: 'fixture',
              role: 'assistant',
              parts: [
                {
                  type: 'reasoning',
                  data: { thinking: 'Thinking about this request.\n'.repeat(180) }
                },
                { type: 'text', data: { text: 'Reasoning answer' } }
              ]
            }
          ])
        }
      >
        Reasoning fixture
      </button>
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
