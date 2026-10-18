import { useEffect, useRef, useState } from 'react'
export type EditorDocument = {
  id: string
  workspaceId: string
  path: string
  name: string
  content: string
  savedContent: string
  revision: string
  image?: string
  original?: string
  source?: string
}
export function useDocuments(onError: (error: unknown) => void) {
  const [documents, setDocuments] = useState<EditorDocument[]>([])
  const [active, setActive] = useState('')
  const [saving, setSaving] = useState(false)
  const latest = useRef(documents)
  const pending = useRef(new Set<string>())
  function commit(update: (items: EditorDocument[]) => EditorDocument[]) {
    const next = update(latest.current)
    latest.current = next
    setDocuments(next)
  }
  const sequence = useRef(0)
  const dirty = documents.some(
    (item) => !item.image && item.original === undefined && item.content !== item.savedContent
  )
  useEffect(() => {
    void window.prime.editorDirty(dirty)
    const prevent = (event: BeforeUnloadEvent) => {
      if (dirty) {
        event.preventDefault()
        event.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', prevent)
    return () => window.removeEventListener('beforeunload', prevent)
  }, [dirty])
  async function open(workspaceId: string, path: string, diff?: any) {
    const request = ++sequence.current
    const id = `${workspaceId}:${path}:${diff?.source || 'file'}`
    if (latest.current.some((item) => item.id === id)) {
      setActive(id)
      return
    }
    try {
      const data = diff || (await window.prime.readFile(workspaceId, path))
      const document: EditorDocument = {
        id,
        workspaceId,
        path,
        name: data.name || path.split(/[\\/]/).pop(),
        content: data.after ?? data.content ?? '',
        savedContent: data.after ?? data.content ?? '',
        revision: data.revision || '',
        image: data.image,
        original: data.before,
        source: data.source
      }
      commit((items) => (items.some((item) => item.id === id) ? items : [...items, document]))
      if (request === sequence.current) setActive(id)
    } catch (error) {
      onError(error)
    }
  }
  function edit(id: string, content: string) {
    commit((items) => items.map((item) => (item.id === id ? { ...item, content } : item)))
  }
  async function save(id: string) {
    const document = latest.current.find((item) => item.id === id)
    if (!document || document.image || document.original !== undefined) return false
    if (pending.current.has(id)) return false
    pending.current.add(id)
    setSaving(true)
    try {
      const result = await window.prime.saveFile(
        document.workspaceId,
        document.path,
        document.content,
        document.revision
      )
      commit((items) =>
        items.map((item) =>
          item.id === id
            ? { ...item, savedContent: document.content, revision: result.revision }
            : item
        )
      )
      window.dispatchEvent(new Event('prime:file-saved'))
      return true
    } catch (error) {
      onError(error)
      return false
    } finally {
      pending.current.delete(id)
      setSaving(pending.current.size > 0)
    }
  }
  function close(id: string) {
    const index = latest.current.findIndex((item) => item.id === id)
    const remaining = latest.current.filter((item) => item.id !== id)
    latest.current = remaining
    setDocuments(remaining)
    setActive((current) =>
      current === id ? remaining[Math.min(index, remaining.length - 1)]?.id || '' : current
    )
  }
  function closeSaved(id: string) {
    const item = latest.current.find((item) => item.id === id)
    if (!item || item.content !== item.savedContent) return false
    close(id)
    return true
  }
  async function reload(id: string) {
    const document = latest.current.find((item) => item.id === id)
    if (!document) return
    try {
      const data = await window.prime.readFile(document.workspaceId, document.path)
      commit((items) =>
        items.map((item) =>
          item.id === id
            ? {
                ...item,
                content: data.content,
                savedContent: data.content,
                revision: data.revision
              }
            : item
        )
      )
    } catch (error) {
      onError(error)
    }
  }
  return { documents, active, setActive, open, edit, save, close, closeSaved, reload, saving }
}
export type Documents = ReturnType<typeof useDocuments>
