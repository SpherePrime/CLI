import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nProvider } from '../../src/renderer/src/i18n'
import {
  shouldShowUpdateScreen,
  UpdateScreen
} from '../../src/renderer/src/features/updates/UpdateScreen'

describe('UpdateScreen', () => {
  it('renders nothing before ready', () => {
    const markup = renderToStaticMarkup(
      <I18nProvider language="ru">
        <UpdateScreen ready={false} busy={false} />
      </I18nProvider>
    )
    expect(markup).toBe('')
  })

  it('shows downloaded and error states even when error omits version', () => {
    expect(shouldShowUpdateScreen(false, { state: 'downloaded', version: '0.4.115' })).toBe(false)
    expect(shouldShowUpdateScreen(true, { state: 'downloaded', version: '0.4.115' })).toBe(true)
    expect(shouldShowUpdateScreen(true, { state: 'error', message: 'failed' })).toBe(true)
    expect(shouldShowUpdateScreen(true, { state: 'current', version: '0.4.115' })).toBe(false)
  })
})
