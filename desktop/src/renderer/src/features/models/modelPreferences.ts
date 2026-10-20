import type { ModelCapability } from './useModelCapabilities'

export type ModelReference = { provider: string; model: string }

export type ModelGroup = {
  id: string
  kind: 'recent' | 'favorites' | 'provider'
  name: string
  models: ModelCapability[]
}

export function modelIdentity(reference: ModelReference) {
  return JSON.stringify([reference.provider, reference.model])
}

function uniqueReferences(references: ModelReference[], limit: number) {
  const identities = new Set<string>()
  return references.filter((reference) => {
    if (!reference?.provider || !reference?.model || identities.size >= limit) return false
    const identity = modelIdentity(reference)
    if (identities.has(identity)) return false
    identities.add(identity)
    return true
  })
}

export function recordRecentModel(current: ModelReference[] | undefined, model: ModelReference) {
  return uniqueReferences([model, ...(current || [])], 8)
}

export function toggleFavoriteModel(current: ModelReference[] | undefined, model: ModelReference) {
  const favorites = uniqueReferences(current || [], 100)
  const identity = modelIdentity(model)
  return favorites.some((reference) => modelIdentity(reference) === identity)
    ? favorites.filter((reference) => modelIdentity(reference) !== identity)
    : uniqueReferences([...favorites, model], 100)
}

export function buildModelGroups(
  models: ModelCapability[],
  recent: ModelReference[] | undefined,
  favorites: ModelReference[] | undefined,
  query: string
): ModelGroup[] {
  const search = query.trim().toLowerCase()
  const available = new Map<string, ModelCapability>()
  for (const model of models) {
    const searchable = `${model.name || ''} ${model.model_id} ${model.provider_id} ${model.provider_name || ''}`
    if (searchable.toLowerCase().includes(search)) {
      available.set(modelIdentity({ provider: model.provider_id, model: model.model_id }), model)
    }
  }
  const groups: ModelGroup[] = []
  for (const [kind, references, limit] of [
    ['recent', recent, 8],
    ['favorites', favorites, 100]
  ] as const) {
    const group: ModelGroup = { id: kind, kind, name: '', models: [] }
    for (const reference of uniqueReferences(references || [], limit)) {
      const identity = modelIdentity(reference)
      const model = available.get(identity)
      if (model) {
        group.models.push(model)
        available.delete(identity)
      }
    }
    if (group.models.length) groups.push(group)
  }
  const providers = new Map<string, ModelGroup>()
  for (const model of available.values()) {
    let group = providers.get(model.provider_id)
    if (!group) {
      group = {
        id: JSON.stringify(['provider', model.provider_id]),
        kind: 'provider',
        name: model.provider_name || model.provider_id,
        models: []
      }
      providers.set(model.provider_id, group)
      groups.push(group)
    }
    group.models.push(model)
  }
  return groups
}
