import { useCallback, useEffect, useRef, useState } from 'react'
import type { FileSuggestion } from './suggestions'

export type MentionFileEntry = {
  name: string
  path: string
  directory: boolean
}

export function extractMentionQuery(text: string): string | null {
  if (!/@[^\s]*$/.test(text)) return null
  return text.split('@').pop()!.toLowerCase()
}

export function toFileSuggestions(
  entries: MentionFileEntry[],
  query: string,
  labels: { file: string; directory: string }
): FileSuggestion[] {
  const needle = query.toLowerCase()
  const filtered = needle
    ? entries.filter(
        (entry) =>
          entry.name.toLowerCase().includes(needle) || entry.path.toLowerCase().includes(needle)
      )
    : entries
  return filtered.slice(0, 8).map((entry) => ({
    label: '@' + entry.path,
    description: entry.directory ? labels.directory : labels.file,
    file: entry.path
  }))
}

export type MentionControllerOptions = {
  debounceMs?: number
  cacheTtlMs?: number
  searchFiles?: (workspaceId: string, query: string, limit: number) => Promise<MentionFileEntry[]>
  listFiles?: (workspaceId: string) => Promise<MentionFileEntry[]>
  labels: { file: string; directory: string }
  onResult: (suggestions: FileSuggestion[]) => void
}

export function createMentionController(options: MentionControllerOptions) {
  const debounceMs = options.debounceMs ?? 120
  const cacheTtlMs = options.cacheTtlMs ?? 15000
  let requestSeq = 0
  let timer: ReturnType<typeof setTimeout> | null = null
  let labels = options.labels
  const cache = new Map<string, { entries: MentionFileEntry[]; at: number }>()

  const cancelTimer = () => {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  const fetchEntries = async (workspaceId: string, query: string): Promise<MentionFileEntry[]> => {
    if (options.searchFiles) {
      try {
        const results = await options.searchFiles(workspaceId, query, 50)
        if (Array.isArray(results)) return results
      } catch {
      }
    }
    if (options.listFiles) {
      try {
        const fallback = await options.listFiles(workspaceId)
        if (Array.isArray(fallback)) return fallback
      } catch {
      }
    }
    return []
  }

  return {
    setLabels(nextLabels: { file: string; directory: string }) {
      labels = nextLabels
    },
    clear() {
      requestSeq++
      cancelTimer()
      options.onResult([])
    },
    invalidate(workspaceId?: string) {
      if (!workspaceId) {
        cache.clear()
        return
      }
      const prefix = `${workspaceId}\0`
      for (const key of cache.keys()) {
        if (key.startsWith(prefix)) cache.delete(key)
      }
    },
    dispose() {
      requestSeq++
      cancelTimer()
    },
    update(workspaceId: string | undefined, value: string) {
      const query = workspaceId ? extractMentionQuery(value) : null
      if (query === null || !workspaceId) {
        requestSeq++
        cancelTimer()
        options.onResult([])
        return
      }
      const seq = ++requestSeq
      cancelTimer()
      const cacheKey = `${workspaceId}\0${query}`
      const cached = cache.get(cacheKey)
      if (cached && Date.now() - cached.at < cacheTtlMs) {
        options.onResult(toFileSuggestions(cached.entries, query, labels))
        return
      }
      timer = setTimeout(() => {
        timer = null
        void fetchEntries(workspaceId, query)
          .then((entries) => {
            if (seq !== requestSeq) return
            if (cache.size >= 100) {
              const firstKey = cache.keys().next().value
              if (firstKey !== undefined) cache.delete(firstKey)
            }
            cache.set(cacheKey, { entries, at: Date.now() })
            options.onResult(toFileSuggestions(entries, query, labels))
          })
          .catch(() => {
            if (seq !== requestSeq) return
            options.onResult([])
          })
      }, debounceMs)
    }
  }
}

export function useFileMentions({
  workspaceId,
  enabled,
  fileLabel,
  directoryLabel,
  busy
}: {
  workspaceId: string | undefined
  enabled: boolean
  fileLabel: string
  directoryLabel: string
  busy: boolean
}) {
  const [suggestions, setSuggestions] = useState<FileSuggestion[]>([])
  const [suggestionIndex, setSuggestionIndex] = useState(0)

  const controllerRef = useRef<ReturnType<typeof createMentionController> | null>(null)
  if (!controllerRef.current) {
    controllerRef.current = createMentionController({
      debounceMs: 120,
      cacheTtlMs: 15000,
      searchFiles: (id, query, limit) =>
        typeof window.prime?.searchFiles === 'function'
          ? window.prime.searchFiles(id, query, limit)
          : Promise.reject(new Error('searchFiles unavailable')),
      listFiles: (id) =>
        typeof window.prime?.files === 'function'
          ? (window.prime.files(id) as Promise<MentionFileEntry[]>)
          : Promise.resolve([]),
      labels: { file: fileLabel, directory: directoryLabel },
      onResult: (next) => {
        setSuggestions(next)
        setSuggestionIndex(0)
      }
    })
  }

  useEffect(() => {
    controllerRef.current?.setLabels({ file: fileLabel, directory: directoryLabel })
  }, [fileLabel, directoryLabel])

  const clearSuggestions = useCallback(() => {
    controllerRef.current?.clear()
    setSuggestionIndex(0)
  }, [])

  const updateMentions = useCallback(
    (value: string) => {
      if (!enabled || !workspaceId) {
        controllerRef.current?.clear()
        return
      }
      controllerRef.current?.update(workspaceId, value)
    },
    [enabled, workspaceId]
  )

  useEffect(() => {
    controllerRef.current?.clear()
  }, [workspaceId, enabled])

  useEffect(() => {
    const onSaved = () => controllerRef.current?.invalidate(workspaceId)
    window.addEventListener('prime:file-saved', onSaved)
    return () => window.removeEventListener('prime:file-saved', onSaved)
  }, [workspaceId])

  const wasBusy = useRef(busy)
  const busyWorkspace = useRef(workspaceId)
  useEffect(() => {
    const prevBusy = wasBusy.current
    const prevWorkspace = busyWorkspace.current
    wasBusy.current = busy
    busyWorkspace.current = workspaceId
    if (prevWorkspace === workspaceId && prevBusy && !busy) {
      controllerRef.current?.invalidate(workspaceId)
    }
  }, [busy, workspaceId])

  useEffect(() => () => controllerRef.current?.dispose(), [])

  return {
    suggestions,
    suggestionIndex,
    setSuggestionIndex,
    updateMentions,
    clearSuggestions
  }
}
