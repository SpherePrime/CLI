import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { ModelPickerRow } from '../../src/renderer/src/features/models/ModelPickerRow'

it('renders selection and favorite as separate labeled buttons with independent disabled states', () => {
  const html = renderToStaticMarkup(
    <ModelPickerRow
      model={{ provider_id: 'provider', provider_name: 'Provider', model_id: 'model', name: 'Model' }}
      active
      favorite
      disabled
      favoriteDisabled={false}
      showProvider
      select={() => {}}
      toggleFavorite={() => {}}
    />
  )
  expect(html).toContain('role="option" aria-selected="true" disabled=""')
  expect(html).toContain('aria-label="Удалить из избранного: Model"')
  expect(html).toContain('aria-pressed="true"')
  expect(html).toContain('Provider')
  expect(html.match(/<button/g)).toHaveLength(2)
  expect(html).toMatch(/<button[^>]*>[\s\S]*?<\/button><button[^>]*class="model-picker-favorite active"/)
})
