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
import { useHistoryPaging } from './useHistoryPaging'
import type { HistoryTiming, PageDirection } from './messageWindow'
import './historyPaging.css'

export function MessageList({
  messages,
  busy,
  waiting = false,
  retrying = false,
  searchOpen = false,
  closeSearch = () => {},
  hasBefore = false,
  hasAfter = false,
  historyLoading = false,
  historyError = '',
  historyTiming,
  loadHistory,
  searchMessages,
  revealMessage
}: {
  messages: Message[]
  busy: boolean
  waiting?: boolean
  retrying?: boolean
  searchOpen?: boolean
  closeSearch?: () => void
  hasBefore?: boolean
  hasAfter?: boolean
  historyLoading?: boolean
  historyError?: string
  historyTiming?: HistoryTiming
  loadHistory?: (direction: PageDirection) => Promise<void>
  searchMessages?: (query: string) => Promise<{ matches: string[]; total: number }>
  revealMessage?: (id: string) => Promise<void>
}) {
  const viewport = useRef<HTMLDivElement>(null)
  const { t } = useI18n()
  const [following, setFollowing] = useState(true)
  const followingRef = useRef(true)
  const leavingBottom = useRef(false)
  const stable = useRef<Message[]>([])
  const timing = useMemo(() => {
    const local = conversationTiming(messages)
    return {
      startedAt: historyTiming?.started_at ? historyTiming.started_at * 1000 : local.startedAt,
      completed: new Map([...local.completed, ...Object.entries(historyTiming?.completed || {})])
    }
  }, [messages, historyTiming])
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
  const localMatches = useMemo(() => findMessages(visible, query), [visible, query])
  const [remoteMatches, setRemoteMatches] = useState<string[]>([])
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const matches = searchMessages ? remoteMatches : localMatches
  useEffect(() => {
    let active = true
    setRemoteMatches([])
    setSearchError('')
    if (!searchOpen || !query.trim() || !searchMessages) {
      setSearching(false)
      return
    }
    setSearching(true)
    const timer = setTimeout(() => {
      void searchMessages(query)
        .then((result) => {
          if (active) setRemoteMatches(result.matches)
        })
        .catch((error) => {
          if (active) setSearchError(String(error.message || error))
        })
        .finally(() => {
          if (active) setSearching(false)
        })
    }, 250)
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [query, searchOpen, searchMessages])
  const activeMatch = Math.min(matchIndex, Math.max(0, matches.length - 1))
  const matchId = searchOpen ? matches[activeMatch] : undefined
  const searchTarget = useRef<{
    id: string
    query: string
    requested: boolean
    positioned: boolean
  } | null>(null)
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
  const paging = useHistoryPaging({
    messages: visible,
    viewport,
    virtualizer,
    settled,
    hasBefore,
    hasAfter,
    loading: historyLoading,
    load: loadHistory,
    stopFollowing: () => {
      followingRef.current = false
      leavingBottom.current = false
      setFollowing(false)
    }
  })
  useEffect(() => {
    if (!searchOpen || !matchId) {
      searchTarget.current = null
      return
    }
    if (searchTarget.current?.id !== matchId || searchTarget.current.query !== query)
      searchTarget.current = { id: matchId, query, requested: false, positioned: false }
    if (searchTarget.current.positioned) return
    followingRef.current = false
    leavingBottom.current = false
    setFollowing(false)
    if (!visible.some((message) => message.id === matchId)) {
      if (!searchTarget.current.requested) {
        searchTarget.current.requested = true
        void revealMessage?.(matchId)
      }
      return
    }
    searchTarget.current.positioned = true
    virtualizer.cancelScroll()
    followingRef.current = false
    leavingBottom.current = false
    setFollowing(false)
    virtualizer.scrollToIndex(
      visible.findIndex((message) => message.id === matchId),
      { align: 'center' }
    )
  }, [searchOpen, matchId, query, visible])
  useLayoutEffect(() => {
    if (paging.restoring.current) return
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
    paging.interact()
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
      {(historyLoading || historyError || searching || searchError) && (
        <div className="history-status" role="status">
          {historyLoading || searching ? t('Загрузка…') : historyError || searchError}
          {historyError && (
            <button
              onClick={() => void paging.page(hasBefore ? 'before' : hasAfter ? 'after' : 'latest')}
            >
              {t('Повторить')}
            </button>
          )}
        </div>
      )}
      <div
        className="conversation"
        style={{ scrollBehavior: 'auto' }}
        ref={viewport}
        tabIndex={0}
        aria-label={t('История чата')}
        onWheel={(event) => {
          takeControl(event.deltaY < 0)
          if (event.deltaY !== 0) paging.onEdge(event.deltaY > 0)
        }}
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
          ) {
            takeControl(false)
            paging.onEdge(true)
          }
        }}
        onScroll={() => {
          paging.onScroll()
          const view = viewport.current!
          const atBottom = !hasAfter && view.scrollHeight - view.scrollTop - view.clientHeight < 24
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
                message.role !== 'user' &&
                previous !== undefined &&
                previous.role !== 'user' &&
                !timing.completed.has(previous.id)
              const fresh =
                (message.created_at || 0) * 1000 > Math.max(mountedAt.current, Date.now() - 4000)
              const last = row.index === visible.length - 1
              return (
                <div
                  key={message.id}
                  data-index={row.index}
                  data-message-id={message.id}
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
          <WorkingIndicator
            busy={busy && !hasAfter}
            waiting={waiting}
            retrying={retrying}
            startedAt={timing.startedAt}
          />
        </div>
      </div>
      {(!following || hasAfter) && settled && (
        <button
          className="jump-bottom"
          disabled={historyLoading}
          onClick={async () => {
            if (hasAfter) await loadHistory?.('latest')
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
