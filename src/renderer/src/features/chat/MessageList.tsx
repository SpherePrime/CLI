import { PrimeLogo } from '../../components/PrimeLogo'
import { useEffect, useRef, useState } from 'react'
import {
  Check,
  ChevronDown,
  Copy,
  FileCode2,
  Terminal,
  Wrench,
  ArrowDown,
  Sparkles
} from 'lucide-react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Message } from '../../../../shared/contracts'
import { textContent } from '../../../../shared/messages'

function ToolCard({ part }: { part: Message['parts'][number] }) {
  const data = part.data
  const isResult = part.type === 'tool_result'
  const isShell = part.type === 'shell_command'
  const content = isShell ? data.output : isResult ? data.content : data.input
  return (
    <details className={`tool-card ${data.is_error ? 'tool-error' : ''}`}>
      <summary>
        <span className="tool-icon">
          {isShell ? (
            <Terminal size={15} />
          ) : /edit|write|read|view/i.test(data.name) ? (
            <FileCode2 size={15} />
          ) : (
            <Wrench size={15} />
          )}
        </span>
        <span>{isShell ? data.command : data.name || 'Инструмент'}</span>
        <small>
          {data.is_error
            ? 'Ошибка'
            : isResult || data.finished || isShell
              ? 'Готово'
              : 'Выполняется'}
        </small>
        {isResult || data.finished || isShell ? (
          <Check size={14} />
        ) : (
          <span className="dot pulse" />
        )}
        <ChevronDown size={13} />
      </summary>
      <pre>{typeof content === 'string' ? content : JSON.stringify(content, null, 2)}</pre>
      {isShell && <small>Код выхода: {data.exit_code}</small>}
    </details>
  )
}
export function MessageList({ messages, busy }: { messages: Message[]; busy: boolean }) {
  const viewport = useRef<HTMLDivElement>(null)
  const bottom = useRef<HTMLDivElement>(null)
  const [following, setFollowing] = useState(true)
  const [copied, setCopied] = useState('')
  useEffect(() => {
    if (following) bottom.current?.scrollIntoView({ block: 'end' })
  }, [messages, busy, following])
  const shown = messages.length > 400 ? messages.slice(-400) : messages
  const [showAll, setShowAll] = useState(false)
  return (
    <div
      className="conversation"
      ref={viewport}
      onScroll={() => {
        const view = viewport.current!
        setFollowing(view.scrollHeight - view.scrollTop - view.clientHeight < 160)
      }}
    >
      <div className="conversation-inner">
        {messages.length > shown.length && !showAll && (
          <button className="quiet older-messages" onClick={() => setShowAll(true)}>
            Показать предыдущие {messages.length - shown.length} сообщений
          </button>
        )}
        {(showAll ? messages : shown)
          .filter((message) => message.role !== 'system')
          .map((message) => (
            <article className={`message ${message.role}`} key={message.id}>
              <div className="message-author">
                {message.role === 'user' ? (
                  <span className="user-avatar">В</span>
                ) : (
                  <span className="prime-avatar">
                    <PrimeLogo />
                  </span>
                )}
                <strong>
                  {message.role === 'user'
                    ? 'Вы'
                    : message.role === 'tool'
                      ? 'Инструмент'
                      : 'Prime'}
                </strong>
                {message.created_at && (
                  <time>
                    {new Date(message.created_at * 1000).toLocaleTimeString('ru', {
                      hour: '2-digit',
                      minute: '2-digit'
                    })}
                  </time>
                )}
              </div>
              <div className="message-body">
                {(message.parts || []).map((part, index) => {
                  if (part.type === 'text' && !part.data.hidden)
                    return (
                      <div className="markdown" key={index}>
                        <Markdown
                          remarkPlugins={[remarkGfm]}
                          components={{
                            a: ({ href, children }) => (
                              <a
                                href={href}
                                onClick={(event) => {
                                  event.preventDefault()
                                  if (href) void window.prime.openExternal(href)
                                }}
                              >
                                {children}
                              </a>
                            )
                          }}
                        >
                          {part.data.text || ''}
                        </Markdown>
                      </div>
                    )
                  if (part.type === 'reasoning' && part.data.thinking)
                    return (
                      <details className="reasoning" key={index}>
                        <summary>
                          <Sparkles size={14} /> Размышления <ChevronDown size={13} />
                        </summary>
                        <div>{part.data.thinking}</div>
                      </details>
                    )
                  if (['tool_call', 'tool_result', 'shell_command'].includes(part.type))
                    return <ToolCard key={index} part={part} />
                  if (part.type === 'finish' && ['error', 'canceled'].includes(part.data.reason))
                    return (
                      <div key={index} className="finish-note">
                        {part.data.message ||
                          (part.data.reason === 'canceled'
                            ? 'Задача остановлена'
                            : 'Ошибка выполнения')}
                      </div>
                    )
                  if (part.type === 'image_url')
                    return (
                      <div key={index} className="attachment-note">
                        Изображение:{' '}
                        {part.data.url?.startsWith('data:') ? (
                          <img src={part.data.url} alt="Вложенное изображение" />
                        ) : (
                          part.data.url
                        )}
                      </div>
                    )
                  if (part.type === 'binary')
                    return (
                      <div key={index} className="attachment-note">
                        <FileCode2 size={14} /> Вложение · {part.data.mime_type}
                      </div>
                    )
                  return null
                })}
              </div>
              {textContent(message) && (
                <button
                  className="copy-message icon"
                  aria-label="Скопировать сообщение"
                  onClick={async () => {
                    await navigator.clipboard.writeText(textContent(message))
                    setCopied(message.id)
                    setTimeout(() => setCopied(''), 1500)
                  }}
                >
                  {copied === message.id ? <Check size={14} /> : <Copy size={14} />}
                </button>
              )}
            </article>
          ))}
        {busy && (
          <div className="working">
            <span className="working-spark">✦</span>
            <span>
              Prime работает<span className="ellipsis">…</span>
            </span>
          </div>
        )}
        <div ref={bottom} />
      </div>
      {!following && (
        <button
          className="jump-bottom"
          onClick={() => {
            setFollowing(true)
            bottom.current?.scrollIntoView({ behavior: 'smooth' })
          }}
        >
          <ArrowDown size={14} /> Новые сообщения
        </button>
      )}
    </div>
  )
}
