import { useEffect, useRef, useState } from 'react'
import type { Preferences } from '../../../../shared/contracts'

export function useChatPermissions({
  projectId,
  sessionId,
  preferences,
  savePreferences,
  onError
}: {
  projectId?: string
  sessionId?: string
  preferences: Preferences
  savePreferences: (patch: Partial<Preferences>) => Promise<void>
  onError: (error: unknown) => void
}) {
  const [draft, setDraft] = useState({ projectId: '', skip: false })
  const pending = useRef(new Map<string, Promise<unknown>>())
  const intended = useRef(new Map<string, boolean>())
  const skip = sessionId
    ? (preferences.skipBySession?.[sessionId] ?? false)
    : draft.projectId === projectId && draft.skip

  function apply(projectId: string, sessionId: string, value: boolean) {
    const key = `${projectId}:${sessionId}`
    const previous = pending.current.get(key) || Promise.resolve()
    const update = previous
      .catch(() => {})
      .then(() => window.prime.call(projectId, 'skip', { sessionId, skip: value }))
    pending.current.set(key, update)
    void update
      .finally(() => {
        if (pending.current.get(key) === update) pending.current.delete(key)
      })
      .catch(() => {})
    return update
  }

  useEffect(() => {
    if (!projectId || !sessionId) return
    setDraft({ projectId, skip: false })
    intended.current.delete(`${projectId}:draft`)
    let active = true
    void apply(projectId, sessionId, skip).catch((error) => {
      if (active) onError(error)
    })
    return () => {
      active = false
    }
  }, [projectId, sessionId, skip])

  async function setSkip(value: boolean) {
    if (!projectId) return
    const key = `${projectId}:${sessionId || 'draft'}`
    intended.current.set(key, value)
    if (!sessionId) {
      setDraft({ projectId, skip: value })
      return
    }
    try {
      await apply(projectId, sessionId, value)
      await savePreferences({ skipBySession: { [sessionId]: value } })
    } catch (error) {
      if (intended.current.get(key) === value) intended.current.delete(key)
      throw error
    }
  }

  async function prepareSession(projectId: string, sessionId: string, value: boolean) {
    const selected =
      intended.current.get(`${projectId}:${sessionId}`) ??
      intended.current.get(`${projectId}:draft`) ??
      value
    await apply(projectId, sessionId, selected)
    await savePreferences({ skipBySession: { [sessionId]: selected } })
  }

  return { skip, setSkip, prepareSession }
}
