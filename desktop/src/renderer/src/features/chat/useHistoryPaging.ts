import { useLayoutEffect, useRef } from 'react'
import type { Virtualizer } from '@tanstack/react-virtual'
import type { Message } from '../../../../shared/contracts'
import type { PageDirection } from './messageWindow'

export function useHistoryPaging({
  messages,
  viewport,
  virtualizer,
  settled,
  hasBefore,
  hasAfter,
  loading,
  load,
  stopFollowing
}: {
  messages: Message[]
  viewport: React.RefObject<HTMLDivElement | null>
  virtualizer: Virtualizer<HTMLDivElement, Element>
  settled: boolean
  hasBefore: boolean
  hasAfter: boolean
  loading: boolean
  load?: (direction: PageDirection) => Promise<void>
  stopFollowing: () => void
}) {
  const anchor = useRef<{ id: string; offset: number; first?: string; last?: string } | null>(null)
  const scrolling = useRef(false)
  const lastTop = useRef(0)
  const interacted = useRef(false)
  const frame = useRef(0)
  const pending = useRef(false)
  function capture() {
    const view = viewport.current
    if (!view) return
    const bounds = view.getBoundingClientRect()
    const rows = Array.from(view.querySelectorAll<HTMLElement>('[data-message-id]'))
    const row = rows.find((element) => element.getBoundingClientRect().bottom > bounds.top + 1)
    if (row?.dataset.messageId)
      anchor.current = {
        id: row.dataset.messageId,
        offset: row.getBoundingClientRect().top - bounds.top,
        first: messages[0]?.id,
        last: messages.at(-1)?.id
      }
  }
  async function page(direction: PageDirection) {
    if (!load || pending.current || loading) return
    pending.current = true
    stopFollowing()
    if (direction !== 'latest') capture()
    else anchor.current = null
    try {
      await load(direction)
    } finally {
      pending.current = false
    }
  }
  useLayoutEffect(() => {
    const saved = anchor.current
    const view = viewport.current
    if (!saved || !view) return
    if (saved.first === messages[0]?.id && saved.last === messages.at(-1)?.id) return
    const index = messages.findIndex((message) => message.id === saved.id)
    if (index < 0) {
      anchor.current = null
      return
    }
    scrolling.current = true
    virtualizer.scrollToIndex(index, { align: 'start' })
    let attempts = 0
    const restore = () => {
      const row = Array.from(view.querySelectorAll<HTMLElement>('[data-message-id]')).find(
        (element) => element.dataset.messageId === saved.id
      )
      if (row)
        view.scrollTop +=
          row.getBoundingClientRect().top - view.getBoundingClientRect().top - saved.offset
      if (++attempts < 6) frame.current = requestAnimationFrame(restore)
      else {
        scrolling.current = false
        anchor.current = null
        lastTop.current = view.scrollTop
      }
    }
    frame.current = requestAnimationFrame(restore)
    return () => cancelAnimationFrame(frame.current)
  }, [messages[0]?.id, messages.at(-1)?.id])
  return {
    page,
    restoring: scrolling,
    interact: () => {
      interacted.current = true
    },
    onEdge: (down: boolean) => {
      const view = viewport.current
      interacted.current = true
      if (!view || !settled || scrolling.current || pending.current || loading) return
      const atBottom = view.scrollHeight - view.scrollTop - view.clientHeight < 2
      if (down && atBottom && hasAfter) void page('after')
      else if (!down && view.scrollTop < 2 && hasBefore) void page('before')
    },
    onScroll: () => {
      const view = viewport.current
      if (!view) return
      const delta = view.scrollTop - lastTop.current
      lastTop.current = view.scrollTop
      if (!settled || !interacted.current || scrolling.current || pending.current || loading) return
      if (delta < 0 && view.scrollTop < 500 && hasBefore) void page('before')
      else if (
        delta > 0 &&
        view.scrollHeight - view.scrollTop - view.clientHeight < 500 &&
        hasAfter
      )
        void page('after')
    }
  }
}
