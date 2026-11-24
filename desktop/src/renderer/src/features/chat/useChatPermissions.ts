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
  const [draft, setDraft] = useState<{ projectId: string; skip: boolean } | null>(null)
  const pending = useRef(new Map<string, Promise<unknown>>())
  const intended = useRef(new Map<string, boolean>())
  const prevProjectId = useRef<string | undefined>(projectId)
  const prevSessionId = useRef<string | undefined>(sessionId)
  const skipBySessionRef = useRef(preferences.skipBySession)
  skipBySessionRef.current = preferences.skipBySession
  const savePreferencesRef = useRef(savePreferences)
  savePreferencesRef.current = savePreferences
  const onErrorRef = useRef(onError)
  onErrorRef.current = onError

  const defaultSkip = Boolean(preferences.defaultSkip)
  const storedSkip = sessionId ? preferences.skipBySession?.[sessionId] : undefined
  const draftForProject = projectId && draft?.projectId === projectId ? draft.skip : undefined
  const transitionedFromDraft =
    Boolean(projectId && sessionId) &&
    prevSessionId.current === undefined &&
    prevProjectId.current === projectId
  const skip = sessionId
    ? (storedSkip ??
      (projectId ? intended.current.get(`${projectId}:${sessionId}`) : undefined) ??
      (transitionedFromDraft && projectId
        ? intended.current.get(`${projectId}:draft`)
        : undefined) ??
      defaultSkip)
    : (draftForProject ?? defaultSkip)

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
    const wasDraft = prevSessionId.current === undefined && prevProjectId.current === projectId
    prevProjectId.current = projectId
    prevSessionId.current = sessionId
    if (!projectId || !sessionId) return
    const draftKey = `${projectId}:draft`
    const draftValue = wasDraft ? intended.current.get(draftKey) : undefined
    const hasStored = skipBySessionRef.current?.[sessionId] !== undefined
    intended.current.delete(draftKey)
    setDraft(null)
    let active = true
    if (draftValue !== undefined && !hasStored) {
      const sessionKey = `${projectId}:${sessionId}`
      intended.current.set(sessionKey, draftValue)
      void apply(projectId, sessionId, draftValue)
        .then(() => savePreferencesRef.current({ skipBySession: { [sessionId]: draftValue } }))
        .catch((error) => {
          if (intended.current.get(sessionKey) === draftValue) intended.current.delete(sessionKey)
          if (active) onErrorRef.current(error)
        })
    } else {
      void apply(projectId, sessionId, skip).catch((error) => {
        if (active) onErrorRef.current(error)
      })
    }
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
    const draftKey = `${projectId}:draft`
    const sessionKey = `${projectId}:${sessionId}`
    const selected =
      intended.current.get(sessionKey) ??
      intended.current.get(draftKey) ??
      value
    intended.current.set(sessionKey, selected)
    intended.current.delete(draftKey)
    await apply(projectId, sessionId, selected)
    await savePreferences({ skipBySession: { [sessionId]: selected } })
  }

  return { skip, setSkip, prepareSession }
}
