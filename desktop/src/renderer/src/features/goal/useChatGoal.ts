import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import { realSessionId } from '../../../../shared/pendingSession'
import { goalPhase, goalProgress, normalizeGoal, type ChatGoal } from '../../../../shared/chatGoal'

export function useChatGoal(w: Workspace) {
  const projectId = w.project?.id
  const sessionId = realSessionId(w.session)
  const busy = Boolean(w.busy)
  const todos = w.session?.todos as
    { content?: string; status?: string; active_form?: string }[] | undefined
  const todoKey = JSON.stringify(todos ?? [])
  const [goal, setGoal] = useState<ChatGoal | null>(null)
  const [saving, setSaving] = useState(false)

  const reload = useCallback(async () => {
    if (!projectId || !sessionId) {
      setGoal(null)
      return
    }
    try {
      setGoal(normalizeGoal(await window.prime.call(projectId, 'goal', { sessionId })))
    } catch {
      setGoal(null)
    }
  }, [projectId, sessionId])

  useEffect(() => {
    void reload()
  }, [reload, busy, todoKey])

  const progress = useMemo(() => goalProgress(todos), [todoKey])
  const phase = goalPhase(goal, progress, busy)

  const save = async (text: string, start: boolean) => {
    const trimmed = text.trim()
    if (!trimmed) return false
    setSaving(true)
    try {
      if (start || !projectId || !sessionId) {
        const sent = await w.send(trimmed, [], { goal: trimmed })
        await reload()
        return sent
      }
      setGoal(normalizeGoal(await w.call('setGoal', { sessionId, text: trimmed })))
      return true
    } catch (error) {
      w.errorHandler(error)
      return false
    } finally {
      setSaving(false)
    }
  }

  const clear = async () => {
    if (!sessionId) return
    try {
      await w.call('clearGoal', { sessionId })
      setGoal(null)
    } catch (error) {
      w.errorHandler(error)
    }
  }

  const resume = async (prompt: string) => {
    await w.send(prompt)
  }

  return {
    goal,
    progress,
    phase,
    saving,
    busy,
    hasSession: Boolean(sessionId),
    stages: todos ?? [],
    save,
    clear,
    resume
  }
}
