import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { PrimeAILimits } from '../../src/shared/contracts'
import { translate } from '../../src/shared/localization'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { PrimeLimitsDetails } from '../../src/renderer/src/features/auth/PrimeLimitsDetails'
import { PrimeLimitsWidget } from '../../src/renderer/src/features/auth/PrimeLimitsWidget'
import {
  formatCompactLimitCount,
  formatResetCountdown,
  formatWindowUsage,
  getLimitLevel,
  getMaxLimitsLevel,
  isUnlimitedLimitWindow
} from '../../src/renderer/src/features/auth/primeLimitsFormat'
import { installPreviewHost } from '../../src/renderer/src/host/installPreviewHost'

function makeLimits(overrides: Partial<PrimeAILimits> = {}): PrimeAILimits {
  return {
    available: true,
    planName: 'Pro',
    cacheWeightPercent: 0,
    fiveHour: {
      usedTokens: 142500,
      limitTokens: 500000,
      usedRequests: 18,
      limitRequests: 120,
      percent: 28.5,
      resetInSeconds: 3 * 3600 + 12 * 60
    },
    weekly: {
      usedTokens: 710000,
      limitTokens: 5000000,
      usedRequests: 86,
      limitRequests: 1000,
      percent: 14.2,
      resetInSeconds: 4 * 86400 + 6 * 3600
    },
    updatedAt: '2026-11-14T10:30:00.000Z',
    ...overrides
  }
}

describe('prime limits formatting helpers', () => {
  it('classifies normal, warn and critical levels accurately', () => {
    expect(getLimitLevel(0)).toBe('normal')
    expect(getLimitLevel(74.9)).toBe('normal')
    expect(getLimitLevel(75)).toBe('warn')
    expect(getLimitLevel(89.9)).toBe('warn')
    expect(getLimitLevel(90)).toBe('critical')
    expect(getLimitLevel(100)).toBe('critical')
  })

  it('ignores unlimited windows when calculating max level', () => {
    expect(
      isUnlimitedLimitWindow({
        usedTokens: 1200,
        limitTokens: 0,
        usedRequests: 4,
        limitRequests: 0,
        percent: 99,
        resetInSeconds: 0
      })
    ).toBe(true)

    const limits = makeLimits({
      fiveHour: {
        usedTokens: 1200,
        limitTokens: 0,
        usedRequests: 4,
        limitRequests: 0,
        percent: 99,
        resetInSeconds: 0
      },
      weekly: {
        usedTokens: 100000,
        limitTokens: 5000000,
        usedRequests: 10,
        limitRequests: 1000,
        percent: 2,
        resetInSeconds: 3600
      }
    })
    expect(getMaxLimitsLevel(limits)).toBe('normal')
  })

  it('formats compact counts, usage strings and reset countdowns in RU and EN', () => {
    const tRu = (s: string, v?: Record<string, string | number>) => translate('ru', s, v)
    const tEn = (s: string, v?: Record<string, string | number>) => translate('en', s, v)

    expect(formatCompactLimitCount(142500)).toBe('143k')
    expect(formatCompactLimitCount(5000000)).toBe('5M')
    expect(formatCompactLimitCount(3200000)).toBe('3.2M')

    expect(
      formatResetCountdown(
        {
          usedTokens: 10,
          limitTokens: 100,
          usedRequests: 1,
          limitRequests: 10,
          percent: 10,
          resetInSeconds: 3 * 3600 + 12 * 60
        },
        tRu
      )
    ).toBe('Сброс через 3ч 12м')

    expect(
      formatResetCountdown(
        {
          usedTokens: 10,
          limitTokens: 100,
          usedRequests: 1,
          limitRequests: 10,
          percent: 10,
          resetInSeconds: 4 * 86400 + 6 * 3600
        },
        tEn
      )
    ).toBe('Resets in 4d 6h')

    expect(
      formatWindowUsage(
        {
          usedTokens: 0,
          limitTokens: 0,
          usedRequests: 0,
          limitRequests: 0,
          percent: 0,
          resetInSeconds: 0
        },
        tRu,
        'ru-RU'
      )
    ).toBe('Без лимита')

    expect(
      formatWindowUsage(
        {
          usedTokens: 0,
          limitTokens: 0,
          usedRequests: 0,
          limitRequests: 0,
          percent: 0,
          resetInSeconds: 0
        },
        tEn,
        'en-US'
      )
    ).toBe('Unlimited')
  })
})

describe('PrimeLimitsWidget and PrimeLimitsDetails', () => {
  it('renders nothing in topbar when limits are missing or unavailable', () => {
    expect(
      renderToStaticMarkup(
        <I18nProvider language="ru">
          <PrimeLimitsWidget limits={null} onRefresh={() => {}} />
        </I18nProvider>
      )
    ).toBe('')

    expect(
      renderToStaticMarkup(
        <I18nProvider language="ru">
          <PrimeLimitsWidget
            limits={{ available: false, cacheWeightPercent: 0 }}
            onRefresh={() => {}}
          />
        </I18nProvider>
      )
    ).toBe('')
  })

  it('renders compact topbar pill with status colors for normal, warn and critical levels', () => {
    const normal = renderToStaticMarkup(
      <I18nProvider language="ru">
        <PrimeLimitsWidget limits={makeLimits()} onRefresh={() => {}} />
      </I18nProvider>
    )
    expect(normal).toContain('topbar-limits is-normal')
    expect(normal).toContain('5ч ')
    expect(normal).toContain('29%')
    expect(normal).toContain('7д ')
    expect(normal).toContain('14%')
    expect(normal).toContain('aria-haspopup="dialog"')
    expect(normal).toContain('aria-expanded="false"')

    const warn = renderToStaticMarkup(
      <I18nProvider language="en">
        <PrimeLimitsWidget
          limits={makeLimits({
            fiveHour: {
              usedTokens: 410000,
              limitTokens: 500000,
              usedRequests: 98,
              limitRequests: 120,
              percent: 82,
              resetInSeconds: 3600
            }
          })}
          onRefresh={() => {}}
        />
      </I18nProvider>
    )
    expect(warn).toContain('topbar-limits is-warn')
    expect(warn).toContain('5h ')
    expect(warn).toContain('82%')
    expect(warn).toContain('7d ')

    const critical = renderToStaticMarkup(
      <I18nProvider language="ru">
        <PrimeLimitsWidget
          limits={makeLimits({
            fiveHour: {
              usedTokens: 500000,
              limitTokens: 500000,
              usedRequests: 120,
              limitRequests: 120,
              percent: 100,
              resetInSeconds: 1800
            }
          })}
          onRefresh={() => {}}
        />
      </I18nProvider>
    )
    expect(critical).toContain('topbar-limits is-critical')
    expect(critical).toContain('100%')
  })

  it('renders PrimeLimitsDetails with percentages only, zero-limit state, refresh state and without cache notice', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <PrimeLimitsDetails
          limits={makeLimits({
            cacheWeightPercent: 15,
            weekly: {
              usedTokens: 0,
              limitTokens: 0,
              usedRequests: 0,
              limitRequests: 0,
              percent: 0,
              resetInSeconds: 0
            }
          })}
          onRefresh={() => {}}
          refreshing={true}
          variant="popover"
        />
      </I18nProvider>
    )
    expect(markup).toContain('PrimeAI Лимиты')
    expect(markup).toContain('Pro')
    expect(markup).toContain('role="progressbar"')
    expect(markup).toContain('aria-valuenow="29"')
    expect(markup).toContain('5-часовой лимит')
    expect(markup).toContain('Недельный лимит')
    expect(markup).toContain('29%')
    expect(markup).toContain('Без лимита')
    expect(markup).not.toContain('ток.')
    expect(markup).not.toContain('запр.')
    expect(markup).not.toContain('Кэш не расходует лимит')
    expect(markup).not.toContain('Учёт кэша')
    expect(markup).toContain('disabled=""')
    expect(markup).toContain('aria-busy="true"')
    expect(markup).toContain('Обновляем…')
  })
})

describe('preview host limits presets', () => {
  function runWithPreviewHost(search: string, fn: () => Promise<void> | void) {
    const previousWindow = (globalThis as { window?: unknown }).window
    const win: Record<string, unknown> = {
      location: { search }
    }
    ;(globalThis as { window?: unknown }).window = win
    return Promise.resolve()
      .then(() => {
        installPreviewHost()
        return fn()
      })
      .finally(() => {
        if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window
        else (globalThis as { window?: unknown }).window = previousWindow
      })
  }

  it('returns default, high and exceeded presets and unavailable when signed out', async () => {
    await runWithPreviewHost('', async () => {
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const limits = await prime.account('limits')
      expect(limits.available).toBe(true)
      expect(limits.planName).toBe('Pro')
      expect(limits.fiveHour.percent).toBe(28.5)
      expect(limits.weekly.percent).toBe(14.2)
    })

    await runWithPreviewHost('?limits=high', async () => {
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const limits = await prime.account('refreshLimits')
      expect(limits.fiveHour.percent).toBe(82)
      expect(limits.weekly.percent).toBe(64)
    })

    await runWithPreviewHost('?limits=exceeded', async () => {
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const limits = await prime.account('limits')
      expect(limits.fiveHour.percent).toBe(100)
      expect(limits.weekly.percent).toBe(95)
    })

    await runWithPreviewHost('?auth=signed-out', async () => {
      const prime = (globalThis as { window: { prime: any } }).window.prime
      const limits = await prime.account('limits')
      expect(limits.available).toBe(false)
    })
  })
})
