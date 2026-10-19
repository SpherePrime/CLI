import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { WindowControlNotice } from '../../src/renderer/src/features/windows/WindowControlBadge'

it('keeps a titled control notice and revoke action visible outside the workspace panel', () => {
  const html = renderToStaticMarkup(
    <WindowControlNotice
      title="Shared editor"
      revoke={() => {}}
      label="Window control"
      revokeLabel="Revoke"
    />
  )
  expect(html).toContain('Shared editor')
  expect(html).toContain('Ctrl+Alt+Escape')
  expect(html).toContain('role="status"')
  expect(html).toContain('Revoke')
  expect(html).toContain('<button')
})
