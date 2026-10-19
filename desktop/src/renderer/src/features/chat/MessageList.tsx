import { useVirtualizer } from '@tanstack/react-virtual'
import { useI18n } from '../../i18n'
import { PrimeLogo } from '../../components/PrimeLogo'
import { ToolCard } from './ToolCard'
import { consolidateToolMessages } from '../../../../shared/toolMessages'
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Check, Copy, FileCode2, ArrowDown } from 'lucide-react'
import { ReasoningBlock } from './ReasoningBlock'
import { clock, useElapsedSeconds } from './elapsed'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Message } from '../../../../shared/contracts'
import { textContent } from '../../../../shared/messages'
import { findMessages } from '../../../../shared/workspaceTools'
import { ConversationSearch } from './ConversationSearch'

export function MessageList({
  messages,
  busy,
  waiting = false,
  searchOpen = false,
  closeSearch = () => {}
}: {
  messages: Message[]
  busy: boolean
  waiting?: boolean
  searchOpen?: boolean
  closeSearch?: () => void
}) {
  const viewport = useRef<HTMLDivElement>(null)
  const { t, locale } = useI18n()
  const [following, setFollowing] = useState(true)
  const followingRef = useRef(true)
  const leavingBottom = useRef(false)
  const [copied, setCopied] = useState('')
  const visible = useMemo(() => consolidateToolMessages(messages), [messages])
  const [query, setQuery] = useState('')
  const [matchIndex, setMatchIndex] = useState(0)
  const matches = useMemo(() => findMessages(visible, query), [visible, query])
  const activeMatch = Math.min(matchIndex, Math.max(0, matches.length - 1))
  const matchId = searchOpen ? matches[activeMatch] : undefined
  const [busySince, setBusySince] = useState<number | null>(null)
  useEffect(() => setBusySince(busy ? Date.now() : null), [busy])
  const working = useElapsedSeconds(busySince)
  const mountedAt = useRef(Date.now())
  const virtualizer = useVirtualizer({
    count: visible.length,
    getScrollElement: () => viewport.current,
    getItemKey: (index) => visible[index].id,
    estimateSize: () => 180,
    overscan: 6
  })
  useEffect(() => {
    if (!searchOpen || !matchId) return
    virtualizer.cancelScroll()
    followingRef.current = false
    leavingBottom.current = false
    setFollowing(false)
    virtualizer.scrollToIndex(
      visible.findIndex((message) => message.id === matchId),
      { align: 'center' }
    )
  }, [searchOpen, matchId, query])
  useLayoutEffect(() => {
    if (followingRef.current && visible.length) virtualizer.scrollToEnd()
    const view = viewport.current
    if (view) {
      const atBottom = view.scrollHeight - view.scrollTop - view.clientHeight < 24
      setFollowing(atBottom)
      if (atBottom && !leavingBottom.current) followingRef.current = true
    }
  }, [messages, virtualizer.getTotalSize()])
  function takeControl(up: boolean) {
    virtualizer.cancelScroll()
    const view = viewport.current
    if (up && view && view.scrollHeight > view.clientHeight + 1) {
      leavingBottom.current = true
      followingRef.current = false
    } else if (!up) leavingBottom.current = false
  }
  return (
    <div className="conversation-panel">
      {searchOpen && (
        <ConversationSearch
          query={query}
          change={(value) => {
            setQuery(value)
            setMatchIndex(0)
          }}
          count={matches.length}
          active={activeMatch}
          move={(delta) =>
            setMatchIndex(
              matches.length ? (activeMatch + delta + matches.length) % matches.length : 0
            )
          }
          close={() => {
            setQuery('')
            closeSearch()
          }}
        />
      )}
      <div
        className="conversation"
        style={{ scrollBehavior: 'auto' }}
        ref={viewport}
        tabIndex={0}
        aria-label={t('История чата')}
        onWheel={(event) => takeControl(event.deltaY < 0)}
        onTouchStart={() => takeControl(true)}
        onPointerDown={(event) => {
          if (event.target === event.currentTarget) takeControl(true)
        }}
        onKeyDown={(event) => {
          if ((event.target as HTMLElement).closest('button,summary,a,input,textarea,select'))
            return
          if (
            ['ArrowUp', 'PageUp', 'Home'].includes(event.key) ||
            (event.key === ' ' && event.shiftKey)
          )
            takeControl(true)
          if (
            ['ArrowDown', 'PageDown', 'End'].includes(event.key) ||
            (event.key === ' ' && !event.shiftKey)
          )
            takeControl(false)
        }}
        onScroll={() => {
          const view = viewport.current!
          const atBottom = view.scrollHeight - view.scrollTop - view.clientHeight < 24
          if (leavingBottom.current && atBottom) {
            setFollowing(true)
            return
          }
          leavingBottom.current = false
          followingRef.current = atBottom
          setFollowing(atBottom)
        }}
      >
        <div className="conversation-inner">
          <div style={{ height: virtualizer.getTotalSize(), position: 'relative', width: '100%' }}>
            {virtualizer.getVirtualItems().map((row) => {
              const message = visible[row.index]
              const previous = visible[row.index - 1]
              const continued =
                message.role !== 'user' && previous !== undefined && previous.role !== 'user'
              const fresh =
                (message.created_at || 0) * 1000 > Math.max(mountedAt.current, Date.now() - 4000)
              const last = row.index === visible.length - 1
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
                  <article
                    className={`message ${message.role} ${continued ? 'continued' : ''} ${fresh ? 'fresh' : ''} ${message.id === matchId ? 'search-match' : ''}`}
                    key={message.id}
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
                            <ReasoningBlock
                              key={index}
                              thinking={part.data.thinking}
                              startedAt={part.data.started_at}
                              finishedAt={part.data.finished_at}
                              live={busy && last && !part.data.finished_at}
                            />
                          )
                        if (['tool_call', 'tool_result', 'shell_command'].includes(part.type))
                          return <ToolCard key={index} part={part} />
                        if (
                          part.type === 'finish' &&
                          ['error', 'canceled'].includes(part.data.reason)
                        )
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
            <div className="working" role="status">
              <span className="working-orb" aria-hidden="true">
                <span />
                <span />
                <span />
              </span>
              <span className="working-label">
                {waiting ? t('Ожидается подтверждение') : t('Prime работает')}
              </span>
              {busy && !waiting && <span className="working-time">{clock(working)}</span>}
            </div>
          )}
        </div>
      </div>
      {!following && (
        <button
          className="jump-bottom"
          onClick={() => {
            leavingBottom.current = false
            followingRef.current = true
            setFollowing(true)
            virtualizer.scrollToEnd()
          }}
        >
          <ArrowDown size={14} /> {t('Новые сообщения')}
        </button>
      )}
    </div>
  )
}
