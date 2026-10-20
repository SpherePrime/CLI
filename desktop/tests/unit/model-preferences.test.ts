import { expect, it } from 'vitest'
import {
  buildModelGroups,
  modelIdentity,
  recordRecentModel,
  toggleFavoriteModel
} from '../../src/renderer/src/features/models/modelPreferences'

it('identifies a model by provider and model without separator collisions', () => {
  expect(modelIdentity({ provider: 'a/b', model: 'c' })).not.toBe(
    modelIdentity({ provider: 'a', model: 'b/c' })
  )
})

it('moves a recently used model to the top and keeps eight unique references', () => {
  const original = Array.from({ length: 10 }, (_, index) => ({ provider: 'p', model: `${index}` }))
  expect(recordRecentModel(original, original[4])).toEqual([
    original[4],
    ...original.slice(0, 4),
    ...original.slice(5, 8)
  ])
  expect(original).toHaveLength(10)
})

it('toggles favorites independently for identical model IDs from different providers', () => {
  const first = { provider: 'first', model: 'same' }
  const second = { provider: 'second', model: 'same' }
  expect(toggleFavoriteModel([first], second)).toEqual([first, second])
  expect(toggleFavoriteModel([first, second], first)).toEqual([second])
  const full = Array.from({ length: 100 }, (_, index) => ({ provider: 'p', model: `${index}` }))
  expect(toggleFavoriteModel(full, second)).toEqual(full)
})

it('places recent before favorites and provider groups with no duplicate or stale rows', () => {
  const models = [
    { provider_id: 'one', model_id: 'same', name: 'Alpha' },
    { provider_id: 'two', model_id: 'same', name: 'Beta' },
    { provider_id: 'one', model_id: 'other', name: 'Gamma' }
  ]
  const recent = [
    { provider: 'two', model: 'same' },
    { provider: 'gone', model: 'stale' }
  ]
  const favorites = [{ provider: 'one', model: 'same' }, ...recent]
  const groups = buildModelGroups([...models, models[0]], recent, favorites, '')
  expect(groups.map((group) => group.kind)).toEqual(['recent', 'favorites', 'provider'])
  expect(groups.map((group) => group.models)).toEqual([[models[1]], [models[0]], [models[2]]])
  expect(buildModelGroups(models, recent, favorites, 'BETA').map((group) => group.models)).toEqual([
    [models[1]]
  ])
})
