import { expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { StartupScreen } from '../../src/renderer/src/components/StartupScreen'

it('shows a branded loading status and immediately releases the interface on success or failure', () => {
  const starting = renderToStaticMarkup(<StartupScreen ready={false} failed={false} />)
  expect(starting).toContain('role="status"')
  expect(starting).toContain('SpherePrime')
  expect(starting).not.toContain('dialog')
  expect(renderToStaticMarkup(<StartupScreen ready failed={false} />)).toBe('')
  expect(renderToStaticMarkup(<StartupScreen ready={false} failed />)).toBe('')
})
