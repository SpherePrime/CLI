import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { PermissionMenu } from '../../src/renderer/src/features/chat/PermissionMenu'

describe('PermissionMenu icon-only trigger', () => {
  it('renders icon-only trigger with accessible mode label and title in Russian', () => {
    const restrictedHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <PermissionMenu
          skip={false}
          disabled={false}
          contextKey="ws-1:sess-1"
          onChange={vi.fn().mockResolvedValue(undefined)}
          onError={vi.fn()}
        />
      </I18nProvider>
    )
    expect(restrictedHtml).toContain('class="permission-trigger "')
    expect(restrictedHtml).toContain('aria-label="Режим доступа: С подтверждением"')
    expect(restrictedHtml).toContain('title="Режим доступа: С подтверждением"')
    expect(restrictedHtml).not.toContain('>С подтверждением<')
    expect(restrictedHtml).not.toContain('menu-chevron')

    const unrestrictedHtml = renderToStaticMarkup(
      <I18nProvider language="ru">
        <PermissionMenu
          skip={true}
          disabled={false}
          contextKey="ws-1:sess-1"
          onChange={vi.fn().mockResolvedValue(undefined)}
          onError={vi.fn()}
        />
      </I18nProvider>
    )
    expect(unrestrictedHtml).toContain('permission-trigger unrestricted')
    expect(unrestrictedHtml).toContain('aria-label="Режим доступа: Полный доступ"')
    expect(unrestrictedHtml).toContain('title="Режим доступа: Полный доступ"')
    expect(unrestrictedHtml).not.toContain('>Полный доступ<')
  })

  it('translates mode tooltip and aria-label to English', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="en">
        <PermissionMenu
          skip={true}
          disabled={false}
          contextKey="ws-1:sess-1"
          onChange={vi.fn().mockResolvedValue(undefined)}
          onError={vi.fn()}
        />
      </I18nProvider>
    )
    expect(html).toContain('aria-label="Access mode: Full access"')
    expect(html).toContain('title="Access mode: Full access"')
    expect(html).not.toContain('>Full access<')
  })
})
