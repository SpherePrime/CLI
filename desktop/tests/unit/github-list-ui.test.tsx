import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it } from 'vitest'
import { GithubList } from '../../src/renderer/src/features/github/GithubList'

const props = {
  tab: 'pulls' as const,
  state: 'all' as const,
  issues: { items: [], error: '' },
  pulls: { items: [], error: '' },
  loading: false,
  limit: 50,
  onTab: () => {},
  onState: () => {},
  onOpen: () => {},
  onRetry: () => {},
  onMore: () => {}
}

it('shows loading or request failures instead of a false empty repository', () => {
  const loading = renderToStaticMarkup(<GithubList {...props} loading />)
  expect(loading).toContain('Загрузка…')
  expect(loading).not.toContain('Нет pull requests')
  expect(loading).not.toContain('<span>0</span>')
  const failed = renderToStaticMarkup(
    <GithubList {...props} pulls={{ items: [], error: 'API unavailable' }} />
  )
  expect(failed).toContain('role="alert"')
  expect(failed).toContain('API unavailable')
  expect(failed).not.toContain('Нет pull requests')
})

it('renders closed and merged pull requests with explicit states and a bounded load-more action', () => {
  const html = renderToStaticMarkup(
    <GithubList
      {...props}
      limit={1}
      pulls={{
        items: [
          {
            number: 7,
            title: 'Merged change',
            state: 'MERGED',
            url: '',
            updatedAt: '',
            headRefName: 'feature',
            baseRefName: 'main',
            isDraft: false
          }
        ],
        error: ''
      }}
    />
  )
  expect(html).toContain('Merged change')
  expect(html).toContain('Объединён')
  expect(html).toContain('Показать ещё')
  expect(html).toContain('1+')
})
