import { useI18n } from '../../i18n'
import { consolidateToolMessages } from '../../../../shared/toolMessages'
import { mergeConsecutiveToolMessages, type MergedMessage } from './toolGroupMerge'
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown } from 'lucide-react'
import type { Message, OrchestratorPhase } from '../../../../shared/contracts'
import type { PreferredDesignImage } from '../designer/designHandoff'
import { MessageItem } from './MessageItem'
import { WorkingIndicator } from './WorkingIndicator'
import { reuseUnchanged } from './stableMessages'
import { findMessages } from '../../../../shared/workspaceTools'
import { ConversationSearch } from './ConversationSearch'
import { conversationTiming, isEndOfTurn, turnCompletedDuration } from './conversationTiming'
import { duration } from './elapsed'
import { useHistoryPaging } from './useHistoryPaging'
import type { HistoryTiming, PageDirection } from './messageWindow'
import './historyPaging.css'

function isNestedScrollWheel(
  target: EventTarget | null,
  container: HTMLElement,
  deltaY: number
): boolean {
  let node = target instanceof HTMLElement ? target : null
  while (node && node !== container) {
    const style = getComputedStyle(node)
    const overflowY = style.overflowY
    if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight + 1) {
      const overscrollY = style.overscrollBehaviorY
      if (overscrollY === 'contain' || overscrollY === 'none') return true
      if (deltaY < 0 && node.scrollTop > 0) return true
      if (deltaY > 0 && node.scrollTop + node.clientHeight < node.scrollHeight - 1) return true
    }
    node = node.parentElement
  }
  return false
}

function isScrollbarPointer(view: HTMLElement, clientX: number): boolean {
  const rect = view.getBoundingClientRect()
  const scale = view.clientWidth > 0 ? rect.width / view.clientWidth : 1
  const scrollbarWidth = Math.max(0, view.offsetWidth - view.clientWidth) * scale
  return scrollbarWidth > 0 && clientX >= rect.right - scrollbarWidth - 2
}

export function MessageList({
  messages,
  busy,
  waiting = false,
  canceling = false,
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
  revealMessage,
  onImplementDesign
}: {
  messages: Message[]
  busy: boolean
  waiting?: boolean
  canceling?: boolean
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
  onImplementDesign?: (
    message: Message,
    allMessages: Message[],
    preferredImage?: PreferredDesignImage
  ) => void
}) {
  const viewport = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const { t } = useI18n()
  const [following, setFollowing] = useState(true)
  const followingRef = useRef(true)
  const leavingBottom = useRef(false)
  const programmaticScroll = useRef(false)
  const programmaticTarget = useRef<number | null>(null)
  const programmaticFrame = useRef(0)
  const lastScrollTop = useRef(0)
  const lastMaxScrollTop = useRef(0)
  const hasAfterRef = useRef(hasAfter)
  hasAfterRef.current = hasAfter
  const stable = useRef<Message[]>([])
  const timing = useMemo(() => {
    const local = conversationTiming(messages, historyTiming?.started_at)
    return {
      startedAt: historyTiming?.started_at ? historyTiming.started_at * 1000 : local.startedAt,
      completed: new Map([...local.completed, ...Object.entries(historyTiming?.completed || {})])
    }
  }, [messages, historyTiming])
  const visible = useMemo(
    () =>
      (stable.current = reuseUnchanged(
        stable.current,
        mergeConsecutiveToolMessages(consolidateToolMessages(messages))
      )),
    [messages]
  )
  const visibleRef = useRef(visible)
  visibleRef.current = visible
  const handleImplementDesign = useMemo(
    () =>
      onImplementDesign
        ? (target: Message, preferredImage?: PreferredDesignImage) =>
            onImplementDesign(target, visibleRef.current, preferredImage)
        : undefined,
    [onImplementDesign]
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
          if (active) setRemoteMatches(Array.isArray(result?.matches) ? result.matches : [])
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
  const markProgrammaticScroll = (view: HTMLElement) => {
    programmaticScroll.current = true
    programmaticTarget.current = view.scrollTop
    lastScrollTop.current = view.scrollTop
    lastMaxScrollTop.current = Math.max(0, view.scrollHeight - view.clientHeight)
    paging.syncTop()
    cancelAnimationFrame(programmaticFrame.current)
    programmaticFrame.current = requestAnimationFrame(() => {
      programmaticScroll.current = false
      programmaticTarget.current = null
    })
  }
  const scrollToBottomNow = () => {
    const view = viewport.current
    if (!view) return
    view.scrollTop = view.scrollHeight
    markProgrammaticScroll(view)
  }
  useEffect(
    () => () => {
      cancelAnimationFrame(programmaticFrame.current)
      programmaticScroll.current = false
      programmaticTarget.current = null
    },
    []
  )
  const paging = useHistoryPaging({
    messages: visible,
    viewport,
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
    const matchesMessage = (message: MergedMessage) =>
      message.id === matchId || Boolean(message._mergedIds?.includes(matchId))
    const matched = visible.find(matchesMessage)
    if (!matched) {
      if (!searchTarget.current.requested) {
        searchTarget.current.requested = true
        void revealMessage?.(matchId)
      }
      return
    }
    searchTarget.current.positioned = true
    followingRef.current = false
    leavingBottom.current = false
    setFollowing(false)
    const view = viewport.current
    const row = Array.from(
      view?.querySelectorAll<HTMLElement>('[data-message-id]') || []
    ).find((element) => element.dataset.messageId === matched.id)
    if (row && view) {
      row.scrollIntoView({ block: 'center', behavior: 'auto' })
      markProgrammaticScroll(view)
    }
  }, [searchOpen, matchId, query, visible])
  useLayoutEffect(() => {
    if (paging.restoring.current) return
    if (hasAfter) followingRef.current = false
    if (!hasAfter && followingRef.current && visible.length) scrollToBottomNow()
    const view = viewport.current
    if (view) {
      const atBottom = !hasAfter && view.scrollHeight - view.scrollTop - view.clientHeight < 24
      setFollowing(atBottom)
      if (atBottom && !leavingBottom.current) followingRef.current = true
    }
  }, [visible, busy, waiting, hasAfter])
  useEffect(() => {
    const view = viewport.current
    const content = contentRef.current
    if (!view || !content) return
    const observer = new ResizeObserver(() => {
      if (paging.restoring.current || !followingRef.current || hasAfterRef.current) return
      scrollToBottomNow()
      setFollowing(true)
    })
    observer.observe(view)
    observer.observe(content)
    return () => observer.disconnect()
  }, [])
  useEffect(() => {
    if (settled || !hasMessages) return
    if (!hasAfterRef.current && followingRef.current) scrollToBottomNow()
    const frame = requestAnimationFrame(() => {
      if (!hasAfterRef.current && followingRef.current) scrollToBottomNow()
      paging.syncTop()
      setSettled(true)
    })
    return () => cancelAnimationFrame(frame)
  }, [settled, hasMessages])
  function takeControl(up: boolean) {
    paging.interact()
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
            <button onClick={() => void paging.retry()}>
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
          if (event.deltaY === 0) return
          if (isNestedScrollWheel(event.target, event.currentTarget, event.deltaY)) return
          takeControl(event.deltaY < 0)
          paging.onEdge(event.deltaY > 0)
        }}
        onPointerDown={(event) => {
          if (event.target === event.currentTarget && isScrollbarPointer(event.currentTarget, event.clientX)) {
            takeControl(true)
          }
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
          const view = viewport.current!
          const top = view.scrollTop
          const maxScrollTop = Math.max(0, view.scrollHeight - view.clientHeight)
          if (paging.restoring.current) {
            lastScrollTop.current = top
            lastMaxScrollTop.current = maxScrollTop
            return
          }
          if (programmaticScroll.current) {
            const expected = programmaticTarget.current
            if (expected !== null && Math.abs(top - expected) <= 2) {
              lastScrollTop.current = top
              lastMaxScrollTop.current = maxScrollTop
              return
            }
            programmaticScroll.current = false
            programmaticTarget.current = null
            cancelAnimationFrame(programmaticFrame.current)
          }
          const clampedByShrink = maxScrollTop < lastMaxScrollTop.current - 1 && top >= maxScrollTop - 2
          const scrolledUp = !clampedByShrink && top < lastScrollTop.current - 1
          lastScrollTop.current = top
          lastMaxScrollTop.current = maxScrollTop
          if (scrolledUp) {
            followingRef.current = false
            leavingBottom.current = true
          }
          const atBottom = !hasAfter && view.scrollHeight - top - view.clientHeight < 24
          if (!atBottom && followingRef.current && !scrolledUp && !leavingBottom.current && !hasAfter) {
            scrollToBottomNow()
            setFollowing(true)
            return
          }
          if (!atBottom && settled) paging.interact()
          paging.onScroll()
          if (leavingBottom.current && atBottom && scrolledUp) {
            setFollowing(true)
            return
          }
          leavingBottom.current = false
          if (!scrolledUp) followingRef.current = atBottom
          setFollowing(atBottom)
        }}
      >
        <div className="conversation-inner" ref={contentRef}>
          <div className={`message-rows ${settled ? '' : 'settling'}`}>
            {visible.map((message, index) => {
              const previous = visible[index - 1]
              const continued =
                !message.is_summary_message &&
                message.role !== 'user' &&
                previous !== undefined &&
                !previous.is_summary_message &&
                previous.role !== 'user' &&
                !isEndOfTurn(visible, index - 1, busy)
              const fresh =
                (message.created_at || 0) * 1000 > Math.max(mountedAt.current, Date.now() - 4000)
              const last = index === visible.length - 1
              const durationTime = isEndOfTurn(visible, index, busy)
                ? turnCompletedDuration(visible, index, timing.completed)
                : undefined
              let previousOrchestratorPhase: OrchestratorPhase | undefined
              for (let i = index - 1; i >= 0; i--) {
                const prev = visible[i]
                if (prev.is_summary_message) continue
                if (prev.role === 'user') break
                if (prev.role === 'assistant') {
                  previousOrchestratorPhase = prev.orchestrator_phase
                  break
                }
              }
              return (
                <div
                  key={message.id}
                  data-index={index}
                  data-message-id={message.id}
                >
                  <MessageItem
                    message={message}
                    continued={continued}
                    fresh={fresh}
                    live={busy && last}
                    matched={message.id === matchId}
                    previousOrchestratorPhase={previousOrchestratorPhase}
                    onImplementDesign={handleImplementDesign}
                  />
                  {durationTime !== undefined && (
                    <div className="working completed-working">
                      {t('Prime работал {time}', {
                        time: duration(durationTime, t)
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
            canceling={canceling}
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
            scrollToBottomNow()
          }}
        >
          <ArrowDown size={14} /> {t('Новые сообщения')}
        </button>
      )}
    </div>
  )
}
