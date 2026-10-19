import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { TerminalPanel } from '../../src/renderer/src/features/terminal/TerminalPanel'
import { I18nProvider } from '../../src/renderer/src/i18n'

it('renders a localized accessible terminal entry point without starting a process on render', () => {
  let requests = 0
  const html = renderToStaticMarkup(
    <I18nProvider language="ru">
      <TerminalPanel
        workspaceId="workspace-a"
        api={async () => {
          requests += 1
          return []
        }}
        subscribe={() => () => {}}
        onError={() => {}}
      />
    </I18nProvider>
  )
  expect(html).toContain('Новый терминал')
  expect(html).toContain('Терминал')
  expect(html).toContain('aria-label=')
  expect(requests).toBe(0)
})
