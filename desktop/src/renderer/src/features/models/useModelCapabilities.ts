import { useEffect, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'

export type ModelCapability = {
  provider_id: string
  provider_name?: string
  model_id: string
  name?: string
  can_reason?: boolean
  reasoning_modes?: string[]
  reasoning_levels?: string[]
  default_reasoning_mode?: string
  context_window?: number
}

const capabilityCache = new Map<string, ModelCapability[]>()

export function useModelCapabilities(workspace: Workspace) {
  const projectId = workspace.project?.id
  const cached = projectId ? capabilityCache.get(projectId) : undefined
  const [models, setModels] = useState<ModelCapability[]>(cached || [])
  const [loading, setLoading] = useState(!cached)
  const [error, setError] = useState('')
  const configKey = JSON.stringify([workspace.config?.providers, workspace.config?.models])
  useEffect(() => {
    let current = true
    setError('')
    if (!projectId) {
      setModels([])
      setLoading(false)
      return
    }
    const known = capabilityCache.get(projectId)
    if (known) setModels(known)
    setLoading(!known)
    void workspace
      .call('models')
      .then((result) => {
        const next: ModelCapability[] = Array.isArray(result) ? result : result?.models || []
        const previous = capabilityCache.get(projectId)
        capabilityCache.set(projectId, next)
        if (current && JSON.stringify(previous) !== JSON.stringify(next)) setModels(next)
      })
      .catch((reason) => {
        if (current) setError(reason?.message || String(reason))
      })
      .finally(() => {
        if (current) setLoading(false)
      })
    return () => {
      current = false
    }
  }, [projectId, configKey])
  return { models, loading, error }
}

export function findCapability(models: ModelCapability[], selected: any) {
  return models.find(
    (model) => model.provider_id === selected?.provider && model.model_id === selected?.model
  )
}
