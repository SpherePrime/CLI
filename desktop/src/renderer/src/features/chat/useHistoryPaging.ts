import { useLayoutEffect, useRef, useState } from 'react'
import type { PageDirection } from './messageWindow'
import type { MergedMessage } from './toolGroupMerge'

export function useHistoryPaging({
  messages,
  viewport,
  settled,
  hasBefore,
  hasAfter,
  loading,
  load,
  stopFollowing
}: {
  messages: MergedMessage[]
  viewport: React.RefObject<HTMLDivElement | null>
  settled: boolean
  hasBefore: boolean
  hasAfter: boolean
  loading: boolean
  load?: (direction: PageDirection) => Promise<void>
  stopFollowing: () => void
}) {
  const anchor = useRef<{
    id: string
    offsetTop: number
    offsetBottom: number
    scrollHeight: number
    first?: string
    last?: string
  } | null>(null)
  const restoring = useRef(false)
  const lastTop = useRef(0)
  const interacted = useRef(false)
  const frame = useRef(0)
  const pending = useRef(false)
  const [pageSettled, setPageSettled] = useState(0)

  function capture() {
    const view = viewport.current
    if (!view) return
    const bounds = view.getBoundingClientRect()
    const rows = Array.from(view.querySelectorAll<HTMLElement>('[data-message-id]'))
    const row = rows.find((element) => element.getBoundingClientRect().bottom > bounds.top + 1)
    if (row?.dataset.messageId) {
      const rect = row.getBoundingClientRect()
      anchor.current = {
        id: row.dataset.messageId,
        offsetTop: rect.top - bounds.top,
        offsetBottom: rect.bottom - bounds.top,
        scrollHeight: view.scrollHeight,
        first: messages[0]?.id,
        last: messages.at(-1)?.id
      }
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
      setPageSettled((count) => count + 1)
    }
  }

  useLayoutEffect(() => {
    const saved = anchor.current
    const view = viewport.current
    if (!saved || !view) return
    if (saved.first === messages[0]?.id && saved.last === messages.at(-1)?.id) {
      if (!pending.current) anchor.current = null
      return
    }

    const matched = messages.find(
      (message) => message.id === saved.id || Boolean(message._mergedIds?.includes(saved.id))
    )
    restoring.current = true

    if (matched) {
      const row = Array.from(view.querySelectorAll<HTMLElement>('[data-message-id]')).find(
        (element) => element.dataset.messageId === matched.id
      )
      if (row) {
        const viewTop = view.getBoundingClientRect().top
        const rect = row.getBoundingClientRect()
        const mergedAtTop = matched.id !== saved.id
        const delta = mergedAtTop
          ? rect.bottom - viewTop - saved.offsetBottom
          : rect.top - viewTop - saved.offsetTop
        if (Math.abs(delta) >= 0.5) {
          view.scrollTop += delta
        }
      } else {
        const heightDelta = view.scrollHeight - saved.scrollHeight
        if (Math.abs(heightDelta) >= 0.5) view.scrollTop += heightDelta
      }
    } else {
      const heightDelta = view.scrollHeight - saved.scrollHeight
      if (Math.abs(heightDelta) >= 0.5) view.scrollTop += heightDelta
    }

    lastTop.current = view.scrollTop
    anchor.current = null
    cancelAnimationFrame(frame.current)
    frame.current = requestAnimationFrame(() => {
      restoring.current = false
      if (viewport.current) lastTop.current = viewport.current.scrollTop
    })
    return () => cancelAnimationFrame(frame.current)
  }, [messages[0]?.id, messages.at(-1)?.id, pageSettled])

  return {
    page,
    restoring,
    interact: () => {
      interacted.current = true
    },
    syncTop: () => {
      if (viewport.current) lastTop.current = viewport.current.scrollTop
    },
    onEdge: (down: boolean) => {
      const view = viewport.current
      interacted.current = true
      if (!view || !settled || restoring.current || pending.current || loading) return
      const atBottom = view.scrollHeight - view.scrollTop - view.clientHeight < 2
      if (down && atBottom && hasAfter) void page('after')
      else if (!down && view.scrollTop < 2 && hasBefore) void page('before')
    },
    onScroll: () => {
      const view = viewport.current
      if (!view) return
      const delta = view.scrollTop - lastTop.current
      lastTop.current = view.scrollTop
      if (!settled || !interacted.current || restoring.current || pending.current || loading) return
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
