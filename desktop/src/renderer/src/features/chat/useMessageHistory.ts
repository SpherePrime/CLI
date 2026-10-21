import { useCallback, useRef, useState } from 'react'
import type { Message } from '../../../../shared/contracts'
import { MessageCache } from './messageCache'
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

export function useMessageHistory() {
  const [state, setState] = useState<MessagePage>(empty)
  const [messagesSessionId, setMessagesSessionId] = useState<string | null>(null)
  const current = useRef<{ id: string | null; page: MessagePage }>({ id: null, page: empty() })
  const cache = useRef(new MessageCache())
  const metadata = useRef(new Map<string, Omit<MessagePage, 'messages'>>())
  const [loading, setLoading] = useState<PageDirection | null>(null)
  const [error, setError] = useState('')
  const request = useRef(0)
  const inFlight = useRef<number | null>(null)
  const commit = useCallback((id: string, page: MessagePage) => {
    current.current = { id, page }
    cache.current.set(id, page.messages)
    const { messages, ...meta } = page
    metadata.current.delete(id)
    metadata.current.set(id, meta)
    while (metadata.current.size > 8) metadata.current.delete(metadata.current.keys().next().value!)
    setState(page)
    setMessagesSessionId(id)
  }, [])
  const select = useCallback(
    (id: string) => {
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
      setState(current.current.page)
      setMessagesSessionId(null)
      return false
    },
    [commit]
  )
  const reset = useCallback(() => {
    request.current++
    inFlight.current = null
    current.current = { id: null, page: empty() }
    setState(current.current.page)
    setMessagesSessionId(null)
    setLoading(null)
    setError('')
  }, [])
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
      commit(message.session_id, updateHistoryMessage(current.current.page, message, action))
    },
    [commit]
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
    ...state,
    messagesSessionId,
    loading,
    error,
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
