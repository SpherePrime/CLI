import { memo, useEffect, useRef, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { MessageAttachment } from './MessageAttachment'
import { MarkdownText } from './MarkdownText'
import { PrimeLogo } from '../../components/PrimeLogo'
import { ToolCard } from './ToolCard'
import { collectToolImages, ToolGallery } from './ToolGallery'
import { ReasoningBlock } from './ReasoningBlock'
import { useI18n } from '../../i18n'
import type { Message } from '../../../../shared/contracts'
import { textContent, visibleMessageText } from '../../../../shared/messages'

export const MessageItem = memo(function MessageItem({
  message,
  continued,
  fresh,
  live,
  matched
}: {
  message: Message
  continued: boolean
  fresh: boolean
  live: boolean
  matched: boolean
}) {
  const { t, locale } = useI18n()
  const [copied, setCopied] = useState(false)
  const copiedTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(copiedTimer.current), [])
  const text = textContent(message)
  if (message.is_summary_message) {
    const finish = (message.parts || []).find((part) => part.type === 'finish')
    if (finish) return null
    return (
      <article className={`message assistant summary-message ${fresh ? 'fresh' : ''} ${matched ? 'search-match' : ''}`}>
        <div className="message-body">
          <div className="compaction-progress" role="status">
            <span className="working-orb" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            <span>{t('Автоматическое сжатие контекста, это может занять несколько минут…')}</span>
          </div>
        </div>
      </article>
    )
  }
  return (
    <article
      className={`message ${message.role} ${continued ? 'continued' : ''} ${fresh ? 'fresh' : ''} ${matched ? 'search-match' : ''}`}
    >
      {!continued && (
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
      )}
      <div className="message-body">
        {(message.parts || []).map((part, index) => {
          if (part.type === 'text' && !part.data.hidden)
            return (
              <div className="markdown" key={index}>
                <MarkdownText text={visibleMessageText(part.data.text || '', message.role)} />
              </div>
            )
          if (part.type === 'reasoning' && part.data.thinking)
            return (
              <ReasoningBlock
                key={index}
                thinking={part.data.thinking}
                startedAt={part.data.started_at}
                finishedAt={part.data.finished_at}
                live={live && !part.data.finished_at}
              />
            )
          if (['tool_call', 'tool_result', 'shell_command'].includes(part.type)) {
            const result =
              part.type === 'tool_result' ? part.data : (part.data.result as unknown)
            const images = collectToolImages(result)
            return (
              <div className="tool-part" key={index}>
                <ToolCard part={part} />
                <ToolGallery images={images} />
              </div>
            )
          }
          if (part.type === 'finish' && ['error', 'canceled'].includes(part.data.reason))
            return (
              <div key={index} className="finish-note">
                {part.data.reason === 'canceled'
                  ? t('Задача остановлена')
                  : part.data.message || t('Ошибка выполнения')}
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
          if (part.type === 'binary') return <MessageAttachment key={index} data={part.data} />
          return null
        })}
      </div>
      {text && (
        <button
          className="copy-message icon"
          aria-label={t('Скопировать сообщение')}
          onClick={async () => {
            await navigator.clipboard.writeText(text)
            setCopied(true)
            clearTimeout(copiedTimer.current)
            copiedTimer.current = setTimeout(() => setCopied(false), 1500)
          }}
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </button>
      )}
    </article>
  )
})
