import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import { WorkingIndicator } from '../../src/renderer/src/features/chat/WorkingIndicator'

describe('WorkingIndicator canceling state', () => {
  it('renders immediate stopping status when canceling is active', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="ru">
        <WorkingIndicator busy canceling waiting={false} startedAt={Date.now() - 5000} />
      </I18nProvider>
    )
    expect(html).toContain('Останавливаю…')
    expect(html).not.toContain('Prime работает')
  })

  it('renders English stopping label when language is en', () => {
    const html = renderToStaticMarkup(
      <I18nProvider language="en">
        <WorkingIndicator busy canceling waiting={false} startedAt={Date.now() - 5000} />
      </I18nProvider>
    )
    expect(html).toContain('Stopping…')
  })
})
