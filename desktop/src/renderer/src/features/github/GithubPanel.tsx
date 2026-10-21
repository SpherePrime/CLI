import { EmptyState } from '../../components/EmptyState'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Bot, ExternalLink, GitPullRequest, Github, Plus, RefreshCw } from 'lucide-react'
import type { GithubIssueDetail, GithubPullDetail, GithubRequest } from '../../../../shared/github'
import { useI18n } from '../../i18n'
import { CreatePull } from './CreatePull'
import { GitHistoryPanel } from './GitHistoryPanel'
import { checksPrompt, issuePrompt, reviewPrompt } from './prompts'
import { GithubList, GithubState } from './GithubList'
import { githubErrorMessage, useGithubRepository } from './useGithubRepository'
import { GithubComments } from './GithubComments'
import '../../styles/automations.css'
import '../../styles/github.css'

type Props = {
  workspaceId: string
  onError(error: unknown): void
  onAgentTask(title: string, prompt: string): Promise<void>
}

type Selection =
  | { kind: 'issue'; detail: GithubIssueDetail }
  | { kind: 'pull'; detail: GithubPullDetail; diff?: string }

export function GithubPanel(props: Props) {
  const { t } = useI18n()
  const [section, setSection] = useState<'history' | 'github'>('history')
  return (
    <div className="github-workspace">
      <div className="github-tabs" role="tablist" aria-label={t('Репозиторий')}>
        <button
          role="tab"
          aria-selected={section === 'history'}
          onClick={() => setSection('history')}
        >
          {t('История коммитов')}
        </button>
        <button
          role="tab"
          aria-selected={section === 'github'}
          onClick={() => setSection('github')}
        >
          GitHub
        </button>
      </div>
      {section === 'history' ? (
        <GitHistoryPanel key={props.workspaceId} workspaceId={props.workspaceId} />
      ) : (
        <GithubWorkspace key={props.workspaceId} {...props} />
      )}
    </div>
  )
}

function GithubWorkspace({ workspaceId, onError, onAgentTask }: Props) {
  const { t } = useI18n()
  const api = useCallback(
    (request: GithubRequest) => window.prime.github(workspaceId, request),
    [workspaceId]
  )
  const { status, state, limit, issues, pulls, loading, error, refresh, changeState, loadMore } =
    useGithubRepository(api)
  const [tab, setTab] = useState<'issues' | 'pulls'>('pulls')
  const [selected, setSelected] = useState<Selection>()
  const [creating, setCreating] = useState(false)
  const [detailBusy, setDetailBusy] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [opening, setOpening] = useState<{ kind: 'issue' | 'pull'; number: number }>()
  const detailSequence = useRef(0)
  const busy = loading || detailBusy || actionBusy
  const [notice, setNotice] = useState('')
  useEffect(() => {
    return () => {
      detailSequence.current++
    }
  }, [])
  const clearDetail = () => {
    detailSequence.current++
    setSelected(undefined)
    setOpening(undefined)
    setDetailError('')
    setDetailBusy(false)
    setCreating(false)
  }
  const open = async (kind: 'issue' | 'pull', number: number) => {
    const sequence = ++detailSequence.current
    setDetailBusy(true)
    setDetailError('')
    setSelected(undefined)
    setOpening({ kind, number })
    setCreating(false)
    try {
      const detail = await api({ action: kind, number })
      if (sequence === detailSequence.current) setSelected({ kind, detail } as Selection)
    } catch (error) {
      if (sequence === detailSequence.current) setDetailError(githubErrorMessage(error))
    } finally {
      if (sequence === detailSequence.current) setDetailBusy(false)
    }
  }
  const run = async (action: () => Promise<void>) => {
    setActionBusy(true)
    try {
      await action()
    } catch (error) {
      onError(error)
    } finally {
      setActionBusy(false)
    }
  }
  if (status && !status.repository)
    return (
      <div className="automation-panel github-panel">
        <EmptyState
          icon={Github}
          title={t('GitHub недоступен')}
          text={t(status.error ?? 'Проект не связан с репозиторием GitHub.')}
          action={{
            label: t('Проверить снова'),
            icon: RefreshCw,
            onClick: () => void refresh(),
            disabled: busy
          }}
        />
      </div>
    )
  return (
    <div className="automation-panel github-panel" aria-label="GitHub">
      <header className="automation-header">
        <div>
          <Github size={17} aria-hidden="true" />
          <h2 title={status?.repository}>{status?.repository ?? 'GitHub'}</h2>
          {status?.account && <span className="automation-count">@{status.account}</span>}
        </div>
        <div className="automation-actions">
          <button aria-label={t('Обновить')} onClick={() => void refresh()} disabled={busy}>
            <RefreshCw size={14} className={busy ? 'spin' : ''} />
          </button>
          <button
            className="automation-primary"
            onClick={() => {
              clearDetail()
              setCreating(true)
            }}
            disabled={busy || !status?.repository}
          >
            <Plus size={14} />
            {t('Pull request')}
          </button>
        </div>
      </header>
      <div className="github-layout">
        <GithubList
          tab={tab}
          state={state}
          limit={limit}
          loading={loading}
          issues={error ? { items: [], error } : issues}
          pulls={error ? { items: [], error } : pulls}
          selected={selected ? { kind: selected.kind, number: selected.detail.number } : opening}
          onTab={(next) => {
            if (next !== tab) {
              clearDetail()
              setTab(next)
            }
          }}
          onState={(next) => {
            clearDetail()
            changeState(next)
          }}
          onOpen={(kind, number) => void open(kind, number)}
          onRetry={() => void refresh()}
          onMore={loadMore}
        />
        <section className="github-detail" aria-busy={detailBusy || actionBusy}>
          {notice && (
            <p className="github-notice" role="status">
              {notice}
            </p>
          )}
          {creating && status && (
            <CreatePull
              status={status}
              api={api}
              onError={onError}
              onCancel={() => setCreating(false)}
              onDone={(url) => {
                setCreating(false)
                setNotice(t('Pull request создан: {value0}', { value0: url }))
                void refresh()
              }}
            />
          )}
          {detailBusy && (
            <p className="launch-placeholder" role="status">
              {t('Загрузка…')}
            </p>
          )}
          {detailError && (
            <div className="github-load-error" role="alert">
              <strong>{t('Не удалось загрузить детали')}</strong>
              <p>{detailError}</p>
              {opening && (
                <button onClick={() => void open(opening.kind, opening.number)}>
                  {t('Повторить')}
                </button>
              )}
            </div>
          )}
          {!creating && !selected && !detailBusy && !detailError && (
            <EmptyState
              compact
              icon={GitPullRequest}
              title={t('Выберите issue или pull request')}
            />
          )}
          {!creating && selected && (
            <article className="github-card">
              <header>
                <h3>
                  #{selected.detail.number} {selected.detail.title}
                </h3>
                <button
                  aria-label={t('Открыть на GitHub')}
                  title={t('Открыть на GitHub')}
                  onClick={() => void window.prime.openExternal(selected.detail.url)}
                >
                  <ExternalLink size={13} />
                </button>
              </header>
              <GithubState state={selected.detail.state} />
              {selected.kind === 'pull' && (
                <p className="github-meta">
                  {selected.detail.headRefName} → {selected.detail.baseRefName}
                  {selected.detail.reviewDecision ? ` · ${selected.detail.reviewDecision}` : ''}
                </p>
              )}
              {selected.detail.body?.trim() && (
                <pre className="github-body">{selected.detail.body.trim()}</pre>
              )}
              {selected.kind === 'issue' && (
                <div className="automation-actions">
                  <button
                    className="automation-primary"
                    disabled={busy}
                    onClick={() =>
                      void run(() =>
                        onAgentTask(
                          `Issue #${selected.detail.number}`,
                          issuePrompt(selected.detail as GithubIssueDetail)
                        )
                      )
                    }
                  >
                    <Bot size={14} />
                    {t('Передать агенту')}
                  </button>
                </div>
              )}
              {selected.kind === 'pull' && (
                <PullSections
                  detail={selected.detail}
                  diff={selected.diff}
                  busy={busy}
                  onDiff={() =>
                    void run(async () => {
                      const diff = await api({ action: 'diff', number: selected.detail.number })
                      setSelected((current) =>
                        current?.kind === 'pull' && current.detail.number === selected.detail.number
                          ? { ...current, diff }
                          : current
                      )
                    })
                  }
                  onReview={() =>
                    void run(() =>
                      onAgentTask(
                        `Review PR #${selected.detail.number}`,
                        reviewPrompt(selected.detail)
                      )
                    )
                  }
                  onChecks={() =>
                    void run(async () => {
                      const logs = await api({
                        action: 'failedLogs',
                        number: selected.detail.number
                      })
                      await onAgentTask(
                        `Checks PR #${selected.detail.number}`,
                        checksPrompt(selected.detail, logs)
                      )
                    })
                  }
                />
              )}
              <GithubComments
                comments={
                  selected.kind === 'pull'
                    ? [
                        ...selected.detail.comments,
                        ...selected.detail.reviews.filter((review) => review.body?.trim()),
                        ...selected.detail.reviewComments
                      ]
                    : selected.detail.comments
                }
              />
            </article>
          )}
        </section>
      </div>
    </div>
  )
}

function PullSections({
  detail,
  diff,
  busy,
  onDiff,
  onReview,
  onChecks
}: {
  detail: GithubPullDetail
  diff?: string
  busy: boolean
  onDiff(): void
  onReview(): void
  onChecks(): void
}) {
  const { t } = useI18n()
  const failed = detail.checks.filter((check) => check.bucket === 'fail')
  const feedback =
    detail.reviewComments.length + detail.reviews.filter((review) => review.body?.trim()).length
  return (
    <>
      <h4>{t('Проверки')}</h4>
      {detail.checksError && (
        <p className="github-load-error" role="alert">
          {detail.checksError}
        </p>
      )}
      <ul className="github-checks">
        {detail.checks.map((check) => (
          <li key={`${check.workflow}-${check.name}`} className={`is-${check.bucket}`}>
            <span className="github-check-dot" />
            {check.workflow ? `${check.workflow} / ` : ''}
            {check.name}
          </li>
        ))}
        {!detail.checks.length && !detail.checksError && (
          <li className="launch-placeholder">{t('Проверок нет')}</li>
        )}
      </ul>
      <h4>
        {t('Изменённые файлы')} <span>{detail.files.length}</span>
      </h4>
      <ul className="github-files">
        {detail.files.slice(0, 40).map((file) => (
          <li key={file.path}>
            <code>{file.path}</code>
            <span className="added">+{file.additions}</span>
            <span className="removed">−{file.deletions}</span>
          </li>
        ))}
      </ul>
      {diff !== undefined && <pre className="github-diff">{diff}</pre>}
      <div className="automation-actions">
        {diff === undefined && (
          <button disabled={busy} onClick={onDiff}>
            {t('Показать изменения')}
          </button>
        )}
        <button disabled={busy || !feedback} onClick={onReview}>
          <Bot size={14} />
          {t('Замечания ревью агенту')} {feedback ? `(${feedback})` : ''}
        </button>
        <button className="automation-primary" disabled={busy || !failed.length} onClick={onChecks}>
          <Bot size={14} />
          {t('Логи упавших проверок агенту')} {failed.length ? `(${failed.length})` : ''}
        </button>
      </div>
    </>
  )
}
