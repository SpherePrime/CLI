import { useVirtualizer } from '@tanstack/react-virtual'
import { useI18n } from '../../i18n'
import { consolidateToolMessages } from '../../../../shared/toolMessages'
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown } from 'lucide-react'
import type { Message } from '../../../../shared/contracts'
import { MessageItem } from './MessageItem'
import { WorkingIndicator } from './WorkingIndicator'
import { reuseUnchanged } from './stableMessages'
import { findMessages } from '../../../../shared/workspaceTools'
import { ConversationSearch } from './ConversationSearch'
import { conversationTiming } from './conversationTiming'
import { duration } from './elapsed'

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
  const { t } = useI18n()
  const [following, setFollowing] = useState(true)
  const followingRef = useRef(true)
  const leavingBottom = useRef(false)
  const stable = useRef<Message[]>([])
  const timing = useMemo(() => conversationTiming(messages), [messages])
  const visible = useMemo(
    () =>
      (stable.current = reuseUnchanged(
        stable.current,
        consolidateToolMessages(messages).filter(
          (message) => !message.is_summary_message || !(message.parts || []).some((part) => part.type === 'finish')
        )
      )),
    [messages]
  )
  const [query, setQuery] = useState('')
  const [matchIndex, setMatchIndex] = useState(0)
  const matches = useMemo(() => findMessages(visible, query), [visible, query])
  const activeMatch = Math.min(matchIndex, Math.max(0, matches.length - 1))
  const matchId = searchOpen ? matches[activeMatch] : undefined
  const mountedAt = useRef(Date.now())
  const [settled, setSettled] = useState(false)
  const hasMessages = visible.length > 0
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
  useEffect(() => {
    if (settled || !hasMessages) return
    let frame = 0
    let stableFrames = 0
    let lastSize = -1
    const startedAt = performance.now()
    const check = () => {
      const size = virtualizer.getTotalSize()
      stableFrames = size === lastSize ? stableFrames + 1 : 0
      lastSize = size
      if (followingRef.current) virtualizer.scrollToEnd()
      if (stableFrames >= 2 || performance.now() - startedAt > 500) setSettled(true)
      else frame = requestAnimationFrame(check)
    }
    frame = requestAnimationFrame(check)
    return () => cancelAnimationFrame(frame)
  }, [settled, hasMessages])
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
          <div
            className={`message-rows ${settled ? '' : 'settling'}`}
            style={{ height: virtualizer.getTotalSize(), position: 'relative', width: '100%' }}
          >
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
                  <MessageItem
                    message={message}
                    continued={continued}
                    fresh={fresh}
                    live={busy && last}
                    matched={message.id === matchId}
                  />
                  {timing.completed.has(message.id) && (
                    <div className="working completed-working">
                      {t('Работал {time}', {
                        time: duration(timing.completed.get(message.id)!, t)
                      })}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
          <WorkingIndicator busy={busy} waiting={waiting} startedAt={timing.startedAt} />
        </div>
      </div>
      {!following && settled && (
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
