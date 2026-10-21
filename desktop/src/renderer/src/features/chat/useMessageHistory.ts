import { useCallback, useRef, useState } from 'react'
import type { Message } from '../../../../shared/contracts'
import { MessageCache } from './messageCache'
import { queueMessageEvent, type MessageEvent } from './messageEvents'
import {
  mergeMessagePage,
  updateHistoryMessage,
  type MessagePage,
  type PageDirection
} from './messageWindow'

const empty = (): MessagePage => ({
  messages: [],
  has_before: false,
  has_after: false,
  timing: { completed: {} }
})

type Structure = {
  id: string | null
  length: number
  hasBefore: boolean
  hasAfter: boolean
  timing: string
}

function structureOf(id: string | null, page: MessagePage): Structure {
  return {
    id,
    length: page.messages.length,
    hasBefore: page.has_before,
    hasAfter: page.has_after,
    timing: JSON.stringify(page.timing)
  }
}

function sameStructure(left: Structure, right: Structure) {
  return (
    left.id === right.id &&
    left.length === right.length &&
    left.hasBefore === right.hasBefore &&
    left.hasAfter === right.hasAfter &&
    left.timing === right.timing
  )
}

export function useMessageHistory() {
  const current = useRef<{ id: string | null; page: MessagePage }>({ id: null, page: empty() })
  const [structure, setStructure] = useState<Structure>(() => structureOf(null, current.current.page))
  const cache = useRef(new MessageCache())
  const metadata = useRef(new Map<string, Omit<MessagePage, 'messages'>>())
  const listeners = useRef(new Set<() => void>())
  const pending = useRef(new Map<string, MessageEvent>())
  const frame = useRef<number | null>(null)
  const [loading, setLoading] = useState<PageDirection | null>(null)
  const [error, setError] = useState('')
  const request = useRef(0)
  const inFlight = useRef<number | null>(null)
  const publish = useCallback((id: string | null, page: MessagePage) => {
    current.current = { id, page }
    for (const listener of listeners.current) listener()
    const next = structureOf(id, page)
    setStructure((previous) => (sameStructure(previous, next) ? previous : next))
  }, [])
  const commit = useCallback(
    (id: string, page: MessagePage) => {
      cache.current.set(id, page.messages)
      const { messages, ...meta } = page
      metadata.current.delete(id)
      metadata.current.set(id, meta)
      while (metadata.current.size > 8) metadata.current.delete(metadata.current.keys().next().value!)
      publish(id, page)
    },
    [publish]
  )
  const cancelLive = useCallback(() => {
    if (frame.current !== null) cancelAnimationFrame(frame.current)
    frame.current = null
    pending.current.clear()
  }, [])
  const flush = useCallback(() => {
    if (frame.current !== null) cancelAnimationFrame(frame.current)
    frame.current = null
    if (!pending.current.size) return
    const events = [...pending.current.values()]
    pending.current.clear()
    const id = current.current.id
    if (!id) return
    let page = current.current.page
    for (const { message, action } of events)
      if (message.session_id === id) page = updateHistoryMessage(page, message, action)
    if (page !== current.current.page) commit(id, page)
  }, [commit])
  const subscribe = useCallback((listener: () => void) => {
    listeners.current.add(listener)
    return () => {
      listeners.current.delete(listener)
    }
  }, [])
  const getPage = useCallback(() => current.current.page, [])
  const select = useCallback(
    (id: string) => {
      cancelLive()
      request.current++
      inFlight.current = null
      setLoading(null)
      setError('')
      const messages = cache.current.get(id)
      const meta = metadata.current.get(id)
      if (messages && meta) {
        commit(id, { ...meta, messages })
        return true
      }
      current.current = { id, page: empty() }
      publish(null, current.current.page)
      current.current.id = id
      return false
    },
    [cancelLive, commit, publish]
  )
  const reset = useCallback(() => {
    cancelLive()
    request.current++
    inFlight.current = null
    publish(null, empty())
    setLoading(null)
    setError('')
  }, [cancelLive, publish])
  const accept = useCallback(
    (id: string, page: MessagePage) => {
      setError('')
      const previous =
        current.current.id === id && current.current.page.messages.length
          ? current.current.page
          : undefined
      commit(id, mergeMessagePage(previous, page, 'latest'))
    },
    [commit]
  )
  const live = useCallback(
    (message: Message, action: string) => {
      if (current.current.id !== message.session_id) return
      queueMessageEvent(pending.current, message.id, { action, message })
      frame.current ??= requestAnimationFrame(() => {
        frame.current = null
        flush()
      })
    },
    [flush]
  )
  const fail = useCallback((id: string, error: unknown) => {
    if (!current.current.id) current.current.id = id
    if (current.current.id === id) setError(String((error as Error).message || error))
  }, [])
  const load = useCallback(
    async (projectId: string, sessionId: string, direction: PageDirection) => {
      if (current.current.id !== sessionId || inFlight.current !== null) return
      const page = current.current.page
      if (
        (direction === 'before' && !page.has_before) ||
        (direction === 'after' && !page.has_after)
      )
        return
      const token = ++request.current
      inFlight.current = token
      setLoading(direction)
      setError('')
      try {
        const result = (await window.prime.call(projectId, 'messagePage', {
          sessionId,
          limit: 60,
          ...(direction === 'before'
            ? { before: page.messages[0]?.id }
            : direction === 'after'
              ? { after: page.messages.at(-1)?.id }
              : {})
        })) as MessagePage
        if (request.current !== token || current.current.id !== sessionId) return
        commit(
          sessionId,
          mergeMessagePage(
            direction === 'latest' ? undefined : current.current.page,
            result,
            direction
          )
        )
      } catch (error) {
        if (request.current === token) setError(String((error as Error).message || error))
      } finally {
        if (request.current === token) {
          inFlight.current = null
          setLoading(null)
        }
      }
    },
    [commit]
  )
  const reveal = useCallback(
    async (projectId: string, sessionId: string, id: string) => {
      if (
        current.current.id !== sessionId ||
        current.current.page.messages.some((message) => message.id === id)
      )
        return
      const token = ++request.current
      inFlight.current = token
      setLoading('latest')
      setError('')
      try {
        const page = (await window.prime.call(projectId, 'messagePage', {
          sessionId,
          around: id,
          limit: 60
        })) as MessagePage
        if (request.current === token && current.current.id === sessionId) commit(sessionId, page)
      } catch (error) {
        if (request.current === token) setError(String((error as Error).message || error))
      } finally {
        if (request.current === token) {
          inFlight.current = null
          setLoading(null)
        }
      }
    },
    [commit]
  )
  return {
    get messages() {
      return current.current.page.messages
    },
    get has_before() {
      return current.current.page.has_before
    },
    get has_after() {
      return current.current.page.has_after
    },
    get timing() {
      return current.current.page.timing
    },
    messagesSessionId: structure.id,
    loading,
    error,
    subscribe,
    getPage,
    flush,
    select,
    reset,
    accept,
    live,
    fail,
    load,
    reveal,
    delete: (id: string) => {
      cache.current.delete(id)
      metadata.current.delete(id)
    }
  }
}

export type MessageHistory = ReturnType<typeof useMessageHistory>
