import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { ModelPickerRow } from '../../src/renderer/src/features/models/ModelPickerRow'
import { buildModelGroups } from '../../src/renderer/src/features/models/modelPreferences'
import type { ModelCapability } from '../../src/renderer/src/features/models/useModelCapabilities'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'

describe('PrimeAI subscription model badges and locking', () => {
  it('renders plan badge for available PrimeAI model and allows selection', () => {
    const model: ModelCapability = {
      provider_id: 'primeai',
      provider_name: 'PrimeAI',
      model_id: 'gpt-5.4-pro',
      name: 'GPT-5.4',
      context_window: 270000,
      plan_label: 'Pro',
      locked: false,
      allowed_tariffs: ['Pro', 'Ultra']
    }

    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelPickerRow
          model={model}
          active={false}
          favorite={false}
          disabled={false}
          favoriteDisabled={false}
          showProvider={false}
          select={vi.fn()}
          toggleFavorite={vi.fn()}
        />
      </I18nProvider>
    )

    expect(html).toContain('model-plan-badge')
    expect(html).toContain('>Pro<')
    expect(html).not.toContain('is-locked')
    expect(html).not.toContain('aria-disabled="true"')
  })

  it('renders locked state, accessible description in RU/EN, and blocks selection', () => {
    const lockedModel: ModelCapability = {
      provider_id: 'primeai',
      provider_name: 'PrimeAI',
      model_id: 'claude-opus-4.5',
      name: 'Claude Opus 4.5',
      context_window: 200000,
      plan_label: 'Ultra',
      locked: true,
      allowed_tariffs: ['Ultra']
    }

    const ruHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <ModelPickerRow
          model={lockedModel}
          active={false}
          favorite={true}
          disabled={false}
          favoriteDisabled={false}
          showProvider={true}
          select={vi.fn()}
          toggleFavorite={vi.fn()}
        />
      </I18nProvider>
    )

    expect(ruHtml).toContain('model-picker-row is-locked')
    expect(ruHtml).toContain('model-plan-badge is-locked')
    expect(ruHtml).toContain('>Ultra<')
    expect(ruHtml).toContain('aria-disabled="true"')
    expect(ruHtml).toContain('disabled=""')
    expect(ruHtml).toContain('Доступно в подписке: Ultra')

    const enHtml = renderToStaticMarkup(
      <I18nProvider language="en">
        <ModelPickerRow
          model={lockedModel}
          active={false}
          favorite={false}
          disabled={false}
          favoriteDisabled={false}
          showProvider={false}
          select={vi.fn()}
          toggleFavorite={vi.fn()}
        />
      </I18nProvider>
    )

    expect(enHtml).toContain('Available with Ultra subscription')
  })

  it('preserves locked and plan_label state inside recent and favorites groups', () => {
    const dynamicModels: ModelCapability[] = [
      {
        provider_id: 'primeai',
        provider_name: 'PrimeAI',
        model_id: 'claude-opus-4.5',
        name: 'Claude Opus 4.5',
        context_window: 200000,
        plan_label: 'Ultra',
        locked: true,
        allowed_tariffs: ['Ultra']
      }
    ]
    const ref = { provider: 'primeai', model: 'claude-opus-4.5' }
    const groups = buildModelGroups(dynamicModels, [ref], [ref], '')

    expect(groups.map((g) => g.kind)).toEqual(['favorites', 'recent', 'provider'])
    for (const group of groups) {
      expect(group.models[0]?.locked).toBe(true)
      expect(group.models[0]?.plan_label).toBe('Ultra')
    }
  })

  it('exposes unlocked Pro and locked Ultra PrimeAI fixtures in preview host when signed in', async () => {
    const previousWindow = (globalThis as { window?: unknown }).window
    try {
      ;(globalThis as { window?: unknown }).window = { location: { search: '?auth=signed-in' } }
      installPreviewHost()
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const signedInModels = await prime.call('ws-prime', 'models')
      const pro = signedInModels.find(
        (m: ModelCapability) => m.provider_id === 'primeai' && m.model_id === 'gpt-5.4-pro'
      )
      const ultra = signedInModels.find(
        (m: ModelCapability) => m.provider_id === 'primeai' && m.model_id === 'claude-opus-4.5'
      )
      expect(pro).toBeDefined()
      expect(pro.plan_label).toBe('Pro')
      expect(pro.locked).toBe(false)
      expect(ultra).toBeDefined()
      expect(ultra.plan_label).toBe('Ultra')
      expect(ultra.locked).toBe(true)

      ;(globalThis as { window?: unknown }).window = { location: { search: '?auth=signed-out' } }
      installPreviewHost()
      const primeSignedOut = (globalThis as { window: { prime: any } }).window.prime
      const signedOutModels = await primeSignedOut.call('ws-prime', 'models')
      expect(signedOutModels.some((m: ModelCapability) => m.provider_id === 'primeai')).toBe(false)
    } finally {
      if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
      else (globalThis as { window?: unknown }).window = previousWindow
    }
  })
})
