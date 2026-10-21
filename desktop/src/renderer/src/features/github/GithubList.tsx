import { CircleDot, GitPullRequest, RefreshCw } from 'lucide-react'
import type { GithubIssue, GithubListState, GithubPull } from '../../../../shared/github'
import { useI18n } from '../../i18n'
import type { GithubListResult } from './useGithubRepository'

export function GithubState({ state }: { state: string }) {
  const { t } = useI18n()
  return (
    <span className={`github-state is-${state.toLowerCase()}`}>
      {t(state === 'MERGED' ? 'Объединён' : state === 'CLOSED' ? 'Закрыт' : 'Открыт')}
    </span>
  )
}

export function GithubList({
  tab,
  state,
  issues,
  pulls,
  loading,
  limit,
  selected,
  onTab,
  onState,
  onOpen,
  onRetry,
  onMore
}: {
  tab: 'issues' | 'pulls'
  state: GithubListState
  issues: GithubListResult<GithubIssue>
  pulls: GithubListResult<GithubPull>
  loading: boolean
  limit: number
  selected?: { kind: 'issue' | 'pull'; number: number }
  onTab(tab: 'issues' | 'pulls'): void
  onState(state: GithubListState): void
  onOpen(kind: 'issue' | 'pull', number: number): void
  onRetry(): void
  onMore(): void
}) {
  const { t } = useI18n()
  const result = tab === 'issues' ? issues : pulls
  const kind = tab === 'issues' ? 'issue' : 'pull'
  const count = (value: GithubListResult<GithubIssue>) =>
    loading
      ? '…'
      : value.error
        ? '—'
        : `${value.items.length}${value.items.length >= limit ? '+' : ''}`
  return (
    <aside className="github-list" aria-busy={loading}>
      <div className="github-tabs" role="tablist" aria-label={t('GitHub')}>
        <button role="tab" aria-selected={tab === 'pulls'} onClick={() => onTab('pulls')}>
          <GitPullRequest size={14} aria-hidden="true" />
          {t('Pull requests')} <span>{count(pulls)}</span>
        </button>
        <button role="tab" aria-selected={tab === 'issues'} onClick={() => onTab('issues')}>
          <CircleDot size={14} aria-hidden="true" />
          {t('Issues')} <span>{count(issues)}</span>
        </button>
      </div>
      <div className="github-filters" role="group" aria-label={t('Состояние')}>
        {(['all', 'open', 'closed'] as const).map((value) => (
          <button
            key={value}
            aria-pressed={state === value}
            onClick={() => {
              if (state !== value) onState(value)
            }}
          >
            {t(value === 'all' ? 'Все' : value === 'open' ? 'Открытые' : 'Закрытые')}
          </button>
        ))}
      </div>
      {loading ? (
        <p className="launch-placeholder" role="status">
          {t('Загрузка…')}
        </p>
      ) : result.error ? (
        <div className="github-load-error" role="alert">
          <strong>{t('Не удалось загрузить список')}</strong>
          <p>{result.error}</p>
          <button onClick={onRetry}>
            <RefreshCw size={14} />
            {t('Повторить')}
          </button>
        </div>
      ) : !result.items.length ? (
        <div className="github-list-empty" role="status">
          <strong>
            {t(
              state === 'open'
                ? 'Нет открытых'
                : state === 'closed'
                  ? 'Нет закрытых'
                  : tab === 'pulls'
                    ? 'Нет pull requests'
                    : 'Нет issues'
            )}
          </strong>
          <p>
            {t(
              state === 'all'
                ? 'В этом репозитории пока нет записей этого типа.'
                : 'Попробуйте другой фильтр состояния.'
            )}
          </p>
        </div>
      ) : (
        <>
          {result.items.map((item) => (
            <button
              key={item.number}
              className={`github-item ${selected?.number === item.number && selected.kind === kind ? 'selected' : ''}`}
              onClick={() => onOpen(kind, item.number)}
            >
              <span className="github-number">#{item.number}</span>
              <span className="github-title">{item.title}</span>
              <small>
                {item.author?.login ? `@${item.author.login}` : ''}
                {'headRefName' in item ? ` · ${item.headRefName}` : ''}
                {'isDraft' in item && item.isDraft ? ` · ${t('черновик')}` : ''}
              </small>
              <GithubState state={item.state} />
            </button>
          ))}
          {result.items.length >= limit && limit < 1000 && (
            <button className="github-load-more" onClick={onMore}>
              {t('Показать ещё')}
            </button>
          )}
          {result.items.length >= 1000 && (
            <p className="launch-placeholder">
              {t('Показаны первые 1000 записей. Уточните фильтр состояния.')}
            </p>
          )}
        </>
      )}
    </aside>
  )
}
