import type { Message } from '../../../../shared/contracts'
import {
  mergeMessagePage,
  type HistoryTiming,
  type MessagePage,
  type PageDirection
} from '../chat/messageWindow'
import { reuseUnchanged } from '../chat/stableMessages'

export const WORKER_PAGE_LIMIT = 60

export type WorkerMessagePageFetcher = (args: {
  limit: number
  before?: string
  after?: string
}) => Promise<Partial<MessagePage>>

export type WorkerMessageHistoryState = {
  messages: Message[]
  hasBefore: boolean
  hasAfter: boolean
  timing: HistoryTiming
  initialLoading: boolean
  pagingLoading: PageDirection | null
  error: string
  pagingError: string
}

const emptyPage = (): MessagePage => ({
  messages: [],
  has_before: false,
  has_after: false,
  timing: { completed: {} }
})

function normalizePage(raw: Partial<MessagePage> | undefined): MessagePage {
  return {
    messages: Array.isArray(raw?.messages) ? raw.messages : [],
    has_before: Boolean(raw?.has_before),
    has_after: Boolean(raw?.has_after),
    timing: raw?.timing
      ? { started_at: raw.timing.started_at, completed: raw.timing.completed || {} }
      : { completed: {} }
  }
}

function sameCompleted(
  left: Record<string, number> | undefined,
  right: Record<string, number> | undefined
): boolean {
  const a = left || {}
  const b = right || {}
  const aKeys = Object.keys(a)
  const bKeys = Object.keys(b)
  if (aKeys.length !== bKeys.length) return false
  return aKeys.every((key) => a[key] === b[key])
}

export function createWorkerMessageHistory({
  fetchPage,
  onChange
}: {
  fetchPage: WorkerMessagePageFetcher
  onChange: (state: WorkerMessageHistoryState) => void
}) {
  let page: MessagePage = emptyPage()
  let initialLoading = true
  let pagingLoading: PageDirection | null = null
  let error = ''
  let pagingError = ''
  let revision = ''
  let requestToken = 0
  let pagingInFlight: number | null = null
  let disposed = false

  const getState = (): WorkerMessageHistoryState => ({
    messages: page.messages,
    hasBefore: page.has_before,
    hasAfter: page.has_after,
    timing: page.timing,
    initialLoading,
    pagingLoading,
    error,
    pagingError
  })

  const emit = () => {
    if (!disposed) onChange(getState())
  }

  const commit = (next: MessagePage): boolean => {
    const reused = reuseUnchanged(page.messages, next.messages)
    const sameMeta =
      page.has_before === next.has_before &&
      page.has_after === next.has_after &&
      page.timing.started_at === next.timing.started_at &&
      sameCompleted(page.timing.completed, next.timing.completed)
    if (reused === page.messages && sameMeta) return false
    page = { ...next, messages: reused }
    return true
  }

  const poll = async (nextRevision: string, active: boolean) => {
    if (disposed || pagingInFlight !== null || page.has_after) return
    if (!initialLoading && !active && revision === nextRevision && !error) return
    const token = ++requestToken
    try {
      const raw = await fetchPage({ limit: WORKER_PAGE_LIMIT })
      if (disposed || token !== requestToken) return
      const incoming = normalizePage(raw)
      const prevInitial = initialLoading
      const prevError = error
      revision = nextRevision
      initialLoading = false
      error = ''
      let changed = prevInitial || prevError !== ''
      if (page.messages.length === 0) {
        changed = commit(mergeMessagePage(undefined, incoming, 'latest')) || changed
      } else if (!page.has_after && !incoming.has_after) {
        changed = commit(mergeMessagePage(page, incoming, 'latest')) || changed
      }
      if (changed) emit()
    } catch (reason) {
      if (disposed || token !== requestToken) return
      initialLoading = false
      error = reason instanceof Error ? reason.message : String(reason)
      emit()
    }
  }

  const load = async (direction: PageDirection) => {
    if (disposed || pagingInFlight !== null) return
    if (direction === 'before' && !page.has_before) return
    if (direction === 'after' && !page.has_after) return
    const token = ++requestToken
    pagingInFlight = token
    pagingLoading = direction
    pagingError = ''
    emit()
    try {
      const args =
        direction === 'before'
          ? { limit: WORKER_PAGE_LIMIT, before: page.messages[0]?.id }
          : direction === 'after'
            ? { limit: WORKER_PAGE_LIMIT, after: page.messages.at(-1)?.id }
            : { limit: WORKER_PAGE_LIMIT }
      const raw = await fetchPage(args)
      if (disposed || token !== requestToken) return
      const incoming = normalizePage(raw)
      const merged = mergeMessagePage(direction === 'latest' ? undefined : page, incoming, direction)
      initialLoading = false
      error = ''
      commit(merged)
    } catch (reason) {
      if (disposed || token !== requestToken) return
      const message = reason instanceof Error ? reason.message : String(reason)
      if (page.messages.length === 0) {
        initialLoading = false
        error = message
      } else {
        pagingError = message
      }
    } finally {
      if (!disposed && pagingInFlight === token) {
        pagingInFlight = null
        pagingLoading = null
        emit()
      }
    }
  }

  const resetRevision = () => {
    revision = ''
  }

  const dispose = () => {
    disposed = true
    requestToken++
    pagingInFlight = null
  }

  return {
    getState,
    poll,
    load,
    resetRevision,
    dispose
  }
}
