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

export function useModelCapabilities(workspace: Workspace) {
  const [models, setModels] = useState<ModelCapability[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const projectId = workspace.project?.id
  useEffect(() => {
    let current = true
    setLoading(true)
    setError('')
    if (!projectId) {
      setModels([])
      setLoading(false)
      return
    }
    void workspace
      .call('models')
      .then((result) => {
        if (current) setModels(Array.isArray(result) ? result : result?.models || [])
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
  }, [projectId, workspace.config])
  return { models, loading, error }
}

export function findCapability(models: ModelCapability[], selected: any) {
  return models.find(
    (model) => model.provider_id === selected?.provider && model.model_id === selected?.model
  )
}
