import { describe, expect, it } from 'vitest'
import type { ModelCapability } from '../../src/renderer/src/features/models/useModelCapabilities'
import {
  buildModelGroups,
  modelIdentity,
  recordRecentModel,
  toggleFavoriteModel
} from '../../src/renderer/src/features/models/modelPreferences'

const models: ModelCapability[] = [
  {
    provider_id: 'primeai',
    provider_name: 'PrimeAI',
    model_id: 'prime-auto',
    name: 'Prime Auto',
    context_window: 200000
  },
  {
    provider_id: 'myapi',
    provider_name: 'Myapi',
    model_id: 'claude-4-sonnet',
    name: 'Claude 4 Sonnet',
    context_window: 200000
  },
  {
    provider_id: 'myapi',
    provider_name: 'Myapi',
    model_id: 'gpt-5',
    name: 'GPT-5',
    context_window: 128000
  },
  {
    provider_id: 'ollama',
    provider_name: 'Ollama',
    model_id: 'qwen2.5-coder',
    name: 'Qwen 2.5 Coder',
    context_window: 32768
  }
]

describe('modelPreferences', () => {
  it('places favorites first, then recent, then provider groups', () => {
    const groups = buildModelGroups(
      models,
      [{ provider: 'myapi', model: 'gpt-5' }],
      [{ provider: 'primeai', model: 'prime-auto' }],
      ''
    )

    expect(groups.map((group) => group.kind)).toEqual([
      'favorites',
      'recent',
      'provider',
      'provider',
      'provider'
    ])
    expect(groups.map((group) => group.id)).toEqual([
      'favorites',
      'recent',
      JSON.stringify(['provider', 'primeai']),
      JSON.stringify(['provider', 'myapi']),
      JSON.stringify(['provider', 'ollama'])
    ])
  })

  it('keeps recent and favorite models visible inside their provider groups', () => {
    const groups = buildModelGroups(
      models,
      [{ provider: 'myapi', model: 'gpt-5' }],
      [{ provider: 'primeai', model: 'prime-auto' }],
      ''
    )

    const primeProvider = groups.find((group) => group.name === 'PrimeAI')
    const myapiProvider = groups.find((group) => group.name === 'Myapi')

    expect(primeProvider?.models.map((item) => item.model_id)).toEqual(['prime-auto'])
    expect(myapiProvider?.models.map((item) => item.model_id)).toEqual([
      'claude-4-sonnet',
      'gpt-5'
    ])
  })

  it('shows a model that is both recent and favorite in favorites, recent, and its provider group', () => {
    const shared = { provider: 'myapi', model: 'claude-4-sonnet' }
    const groups = buildModelGroups(models, [shared], [shared], '')

    expect(groups[0]?.kind).toBe('favorites')
    expect(groups[0]?.models.map((item) => item.model_id)).toEqual(['claude-4-sonnet'])

    expect(groups[1]?.kind).toBe('recent')
    expect(groups[1]?.models.map((item) => item.model_id)).toEqual(['claude-4-sonnet'])

    const myapiProvider = groups.find((group) => group.name === 'Myapi')
    expect(myapiProvider?.models.map((item) => item.model_id)).toEqual([
      'claude-4-sonnet',
      'gpt-5'
    ])
  })

  it('filters favorites, recent, and provider groups when searching', () => {
    const groups = buildModelGroups(
      models,
      [
        { provider: 'myapi', model: 'claude-4-sonnet' },
        { provider: 'myapi', model: 'gpt-5' }
      ],
      [
        { provider: 'primeai', model: 'prime-auto' },
        { provider: 'myapi', model: 'claude-4-sonnet' }
      ],
      'claude'
    )

    expect(groups.map((group) => group.kind)).toEqual(['favorites', 'recent', 'provider'])
    expect(groups[0]?.models.map((item) => item.model_id)).toEqual(['claude-4-sonnet'])
    expect(groups[1]?.models.map((item) => item.model_id)).toEqual(['claude-4-sonnet'])
    expect(groups[2]?.name).toBe('Myapi')
    expect(groups[2]?.models.map((item) => item.model_id)).toEqual(['claude-4-sonnet'])
  })

  it('returns empty list when search matches no models', () => {
    const groups = buildModelGroups(
      models,
      [{ provider: 'myapi', model: 'gpt-5' }],
      [{ provider: 'primeai', model: 'prime-auto' }],
      'nonexistent-model'
    )
    expect(groups).toEqual([])
  })

  it('deduplicates and caps recent models and toggles favorites cleanly', () => {
    const ref = { provider: 'myapi', model: 'gpt-5' }
    const nextRecent = recordRecentModel(
      [
        { provider: 'primeai', model: 'prime-auto' },
        ref,
        { provider: 'ollama', model: 'qwen2.5-coder' }
      ],
      ref
    )
    expect(nextRecent.map(modelIdentity)).toEqual([
      modelIdentity(ref),
      modelIdentity({ provider: 'primeai', model: 'prime-auto' }),
      modelIdentity({ provider: 'ollama', model: 'qwen2.5-coder' })
    ])

    const added = toggleFavoriteModel([], ref)
    expect(added).toEqual([ref])
    const removed = toggleFavoriteModel(added, ref)
    expect(removed).toEqual([])
  })

  it('falls back safely when a model payload lacks provider_name or provider_id', () => {
    const groups = buildModelGroups(
      [{ provider_id: '', model_id: 'fallback-model', name: 'Fallback' }],
      undefined,
      undefined,
      ''
    )
    expect(groups).toHaveLength(1)
    expect(groups[0]?.name).toBe('unknown')
  })
})
