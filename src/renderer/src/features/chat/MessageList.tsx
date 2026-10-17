import { useVirtualizer } from '@tanstack/react-virtual'
import { useI18n } from '../../i18n'
import { PrimeLogo } from '../../components/PrimeLogo'
import { useLayoutEffect, useMemo, useRef, useState } from 'react'
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
  const { t } = useI18n()
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
        <span>{isShell ? data.command : data.name || t('Инструмент')}</span>
        <small>
          {data.is_error
            ? t('Ошибка')
            : isResult || data.finished || isShell
              ? t('Готово')
              : t('Выполняется')}
        </small>
        {isResult || data.finished || isShell ? (
          <Check size={14} />
        ) : (
          <span className="dot pulse" />
        )}
        <ChevronDown size={13} />
      </summary>
      <pre>{typeof content === 'string' ? content : JSON.stringify(content, null, 2)}</pre>
      {isShell && <small>{t('Код выхода: {code}', { code: data.exit_code })}</small>}
    </details>
  )
}
export function MessageList({
  messages,
  busy,
  waiting = false
}: {
  messages: Message[]
  busy: boolean
  waiting?: boolean
}) {
  const viewport = useRef<HTMLDivElement>(null)
  const { t, locale } = useI18n()
  const [following, setFollowing] = useState(true)
  const [copied, setCopied] = useState('')
  const visible = useMemo(() => messages.filter((message) => message.role !== 'system'), [messages])
  const virtualizer = useVirtualizer({
    count: visible.length,
    getScrollElement: () => viewport.current,
    getItemKey: (index) => visible[index].id,
    estimateSize: () => 180,
    overscan: 6
  })
  useLayoutEffect(() => {
    if (following && visible.length) virtualizer.scrollToIndex(visible.length - 1, { align: 'end' })
    const view = viewport.current
    if (view && view.scrollHeight - view.scrollTop - view.clientHeight < 160) setFollowing(true)
  }, [messages, following, virtualizer.getTotalSize()])
  return (
    <div
      className="conversation"
      style={{ scrollBehavior: 'auto' }}
      ref={viewport}
      onScroll={() => {
        const view = viewport.current!
        setFollowing(view.scrollHeight - view.scrollTop - view.clientHeight < 160)
      }}
    >
      <div className="conversation-inner">
        <div style={{ height: virtualizer.getTotalSize(), position: 'relative', width: '100%' }}>
          {virtualizer.getVirtualItems().map((row) => {
            const message = visible[row.index]
            return (
              <div
                key={message.id}
                data-index={row.index}
                ref={virtualizer.measureElement}
                style={{
                  display: 'flow-root',
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${row.start}px)`
                }}
              >
                <article className={`message ${message.role}`} key={message.id}>
                  <div className="message-author">
                    {message.role === 'user' ? (
                      <span className="user-avatar">{t('Вы').slice(0, 1)}</span>
                    ) : (
                      <span className="prime-avatar">
                        <PrimeLogo />
                      </span>
                    )}
                    <strong>
                      {message.role === 'user'
                        ? t('Вы')
                        : message.role === 'tool'
                          ? t('Инструмент')
                          : 'Prime'}
                    </strong>
                    {message.created_at && (
                      <time>
                        {new Date(message.created_at * 1000).toLocaleTimeString(locale, {
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
                              <Sparkles size={14} /> {t('Размышления')} <ChevronDown size={13} />
                            </summary>
                            <div>{part.data.thinking}</div>
                          </details>
                        )
                      if (['tool_call', 'tool_result', 'shell_command'].includes(part.type))
                        return <ToolCard key={index} part={part} />
                      if (
                        part.type === 'finish' &&
                        ['error', 'canceled'].includes(part.data.reason)
                      )
                        return (
                          <div key={index} className="finish-note">
                            {part.data.message ||
                              (part.data.reason === 'canceled'
                                ? t('Задача остановлена')
                                : t('Ошибка выполнения'))}
                          </div>
                        )
                      if (part.type === 'image_url')
                        return (
                          <div key={index} className="attachment-note">
                            {t('Изображение:')}{' '}
                            {part.data.url?.startsWith('data:') ? (
                              <img src={part.data.url} alt={t('Вложенное изображение')} />
                            ) : (
                              part.data.url
                            )}
                          </div>
                        )
                      if (part.type === 'binary')
                        return (
                          <div key={index} className="attachment-note">
                            <FileCode2 size={14} /> {t('Вложение ·')} {part.data.mime_type}
                          </div>
                        )
                      return null
                    })}
                  </div>
                  {textContent(message) && (
                    <button
                      className="copy-message icon"
                      aria-label={t('Скопировать сообщение')}
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
              </div>
            )
          })}
        </div>
        {(busy || waiting) && (
          <div className="working">
            <span className="working-spark">✦</span>
            <span>
              {waiting ? t('Ожидается подтверждение') : t('Prime работает')}
              <span className="ellipsis">…</span>
            </span>
          </div>
        )}
      </div>
      {!following && (
        <button
          className="jump-bottom"
          onClick={() => {
            setFollowing(true)
            virtualizer.scrollToIndex(visible.length - 1, { align: 'end' })
          }}
        >
          <ArrowDown size={14} /> {t('Новые сообщения')}
        </button>
      )}
    </div>
  )
}
