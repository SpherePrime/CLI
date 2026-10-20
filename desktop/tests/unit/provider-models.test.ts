import { describe, expect, it } from 'vitest'
import { normalizeProviderModels } from '../../src/renderer/src/features/settings/providerModels'

const t = (key: string, values?: Record<string, string | number>) =>
  key.replace(/\{(\w+)\}/g, (_, name) => String(values?.[name] ?? name))

describe('provider model limits', () => {
  it('accepts partial catalogs, automatic limits and clears empty overrides', () => {
    const models = [
      { id: 'discovered', context_window: '', supports_attachments: true },
      { id: 'auto', context_window: 0, default_max_tokens: 0 }
    ]
    expect(normalizeProviderModels(models, t)).toEqual([
      { id: 'discovered', supports_attachments: true },
      { id: 'auto', context_window: 0, default_max_tokens: 0 }
    ])
    expect(models[0].context_window).toBe('')
  })
  it('identifies the model with invalid or contradictory limits', () => {
    expect(() => normalizeProviderModels([{ id: 'bad', context_window: -1 }], t)).toThrow('bad')
    expect(() => normalizeProviderModels([{ id: 'bad', default_max_tokens: 1.5 }], t)).toThrow(
      'bad'
    )
    expect(() =>
      normalizeProviderModels([{ id: 'bad', context_window: 10, default_max_tokens: 20 }], t)
    ).toThrow('превышает')
  })
})
