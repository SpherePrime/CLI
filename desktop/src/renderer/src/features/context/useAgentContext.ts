import { useEffect, useRef, useState } from 'react'
import type { Preferences, Project } from '../../../../shared/contracts'
import {
  emptyAgentContext,
  mergeContextSource,
  type AgentContext,
  type ContextSource
} from '../../../../shared/agentContext'
import { translate } from '../../i18n'

export type ContextSourceEvent = { workspaceId: string; source: ContextSource }

export function addAgentSource(workspaceId: string, source: ContextSource) {
  window.dispatchEvent(
    new CustomEvent<ContextSourceEvent>('prime:context-source', { detail: { workspaceId, source } })
  )
}

export function useAgentContext(
  project: Project | null,
  preferences: Preferences,
  save: (patch: Partial<Preferences>) => Promise<void>,
  onError: (error: unknown) => void
) {
  const pending = useRef(new Map<string, AgentContext>())
  const [, render] = useState(0)
  const context =
    pending.current.get(project?.path || '') ||
    preferences.agentContexts?.[project?.path || ''] ||
    emptyAgentContext()
  const latest = useRef({ project, preferences, context, save, onError })
  latest.current = { project, preferences, context, save, onError }
  async function updateContext(next: AgentContext) {
    const current = latest.current
    if (!current.project) return
    const path = current.project.path
    pending.current.set(path, next)
    render((value) => value + 1)
    try {
      await current.save({ agentContexts: { [path]: next } })
    } finally {
      if (pending.current.get(path) === next) pending.current.delete(path)
      render((value) => value + 1)
    }
  }
  useEffect(() => {
    const handle = (event: Event) => {
      const { workspaceId, source } = (event as CustomEvent<ContextSourceEvent>).detail
      const current = latest.current
      if (workspaceId !== current.project?.id) return
      try {
        if (source.kind === 'selection' && (source.content?.length || 0) > 12000)
          throw new Error(
            translate(current.preferences.language, 'Выберите фрагмент до 12 000 символов')
          )
        const next = mergeContextSource(current.context, source)
        current.context = next
        void updateContext(next).catch(current.onError)
      } catch (error) {
        current.onError(error)
      }
    }
    window.addEventListener('prime:context-source', handle)
    return () => window.removeEventListener('prime:context-source', handle)
  }, [])
  return { context, updateContext }
}
