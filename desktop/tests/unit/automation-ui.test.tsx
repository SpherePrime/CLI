import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { AutomationPanel } from '../../src/renderer/src/features/automations/AutomationPanel'
import { AutomationEditor } from '../../src/renderer/src/features/automations/AutomationEditor'
import { I18nProvider } from '../../src/renderer/src/i18n'

it('renders the running-app requirement and never starts automation while rendering', () => {
  let requests = 0
  const html = renderToStaticMarkup(
    <I18nProvider language="ru">
      <AutomationPanel
        root="project"
        api={async () => {
          requests++
          return { root: 'project', rules: [], runs: [] }
        }}
        subscribe={() => () => {}}
        onError={() => {}}
      />
    </I18nProvider>
  )
  expect(html).toContain('Приложение должно оставаться запущенным')
  expect(html).toContain('Новая автоматизация')
  expect(html).toContain('aria-label=')
  expect(requests).toBe(0)
})

it('exposes required title and prompt with real schedule controls', () => {
  const html = renderToStaticMarkup(
    <I18nProvider>
      <AutomationEditor busy={false} onSave={async () => {}} onCancel={() => {}} />
    </I18nProvider>
  )
  expect(html).toContain('required=""')
  expect(html).toContain('min="1"')
  expect(html).toContain('Изменение файлов')
  expect(html).toContain('Ошибка запуска')
  expect(html).toContain('Местное время')
})
