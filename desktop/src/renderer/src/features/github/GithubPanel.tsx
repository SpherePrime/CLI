import { EmptyState } from '../../components/EmptyState'
import { useCallback, useEffect, useState } from 'react'
import { Bot, CircleDot, ExternalLink, GitPullRequest, Github, Plus, RefreshCw } from 'lucide-react'
import type {
  GithubIssue,
  GithubIssueDetail,
  GithubPull,
  GithubPullDetail,
  GithubRequest,
  GithubStatus
} from '../../../../shared/github'
import { useI18n } from '../../i18n'
import { CreatePull } from './CreatePull'
import { checksPrompt, issuePrompt, reviewPrompt } from './prompts'
import '../../styles/automations.css'
import '../../styles/github.css'

type Props = {
  workspaceId: string
  onError(error: unknown): void
  onAgentTask(title: string, prompt: string): Promise<void>
}

type Selection = { kind: 'issue'; detail: GithubIssueDetail } | { kind: 'pull'; detail: GithubPullDetail; diff?: string }

export function GithubPanel(props: Props) {
  return <GithubWorkspace key={props.workspaceId} {...props} />
}

function GithubWorkspace({ workspaceId, onError, onAgentTask }: Props) {
  const { t } = useI18n()
  const api = useCallback((request: GithubRequest) => window.prime.github(workspaceId, request), [workspaceId])
  const [status, setStatus] = useState<GithubStatus>()
  const [tab, setTab] = useState<'issues' | 'pulls'>('pulls')
  const [issues, setIssues] = useState<GithubIssue[]>([])
  const [pulls, setPulls] = useState<GithubPull[]>([])
  const [selected, setSelected] = useState<Selection>()
  const [creating, setCreating] = useState(false)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const refresh = useCallback(async () => {
    setBusy(true)
    try {
      const next: GithubStatus = await api({ action: 'status' })
      setStatus(next)
      if (next.repository) {
        const [issueList, pullList] = await Promise.all([api({ action: 'issues' }), api({ action: 'pulls' })])
        setIssues(issueList)
        setPulls(pullList)
      }
    } catch (error) {
      onError(error)
    } finally {
      setBusy(false)
    }
  }, [api, onError])
  useEffect(() => {
    void refresh()
  }, [refresh])
  const open = async (kind: 'issue' | 'pull', number: number) => {
    setBusy(true)
    try {
      setSelected({ kind, detail: await api({ action: kind, number }) } as Selection)
    } catch (error) {
      onError(error)
    } finally {
      setBusy(false)
    }
  }
  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    try {
      await action()
    } catch (error) {
      onError(error)
    } finally {
      setBusy(false)
    }
  }
  if (status && !status.repository)
    return (
      <div className="automation-panel github-panel">
        <EmptyState
          icon={Github}
          title={t('GitHub недоступен')}
          text={t(status.error ?? 'Проект не связан с репозиторием GitHub.')}
          action={{ label: t('Проверить снова'), icon: RefreshCw, onClick: () => void refresh(), disabled: busy }}
        />
      </div>
    )
  const list = tab === 'issues' ? issues : pulls
  return (
    <div className="automation-panel github-panel" aria-label="GitHub">
      <header className="automation-header">
        <div>
          <Github size={17} aria-hidden="true" />
          <h2>{status?.repository ?? 'GitHub'}</h2>
          {status?.account && <span className="automation-count">@{status.account}</span>}
        </div>
        <div className="automation-actions">
          <button aria-label={t('Обновить')} onClick={() => void refresh()} disabled={busy}>
            <RefreshCw size={14} className={busy ? 'spin' : ''} />
          </button>
          <button className="automation-primary" onClick={() => setCreating(true)} disabled={busy || !status}>
            <Plus size={14} />
            {t('Pull request')}
          </button>
        </div>
      </header>
      <div className="github-layout">
        <aside className="github-list">
          <div className="github-tabs" role="tablist">
            <button role="tab" aria-selected={tab === 'pulls'} onClick={() => setTab('pulls')}>
              <GitPullRequest size={13} />
              {t('Pull requests')} <span>{pulls.length}</span>
            </button>
            <button role="tab" aria-selected={tab === 'issues'} onClick={() => setTab('issues')}>
              <CircleDot size={13} />
              {t('Issues')} <span>{issues.length}</span>
            </button>
          </div>
          {!list.length && <p className="launch-placeholder">{t(busy ? 'Загрузка…' : 'Нет открытых')}</p>}
          {list.map((item) => (
            <button
              key={item.number}
              className={`github-item ${selected?.detail.number === item.number && selected.kind === (tab === 'issues' ? 'issue' : 'pull') ? 'selected' : ''}`}
              onClick={() => void open(tab === 'issues' ? 'issue' : 'pull', item.number)}
            >
              <span className="github-number">#{item.number}</span>
              <span className="github-title">{item.title}</span>
              <small>
                {item.author?.login}
                {'headRefName' in item ? ` · ${(item as GithubPull).headRefName}` : ''}
                {'isDraft' in item && (item as GithubPull).isDraft ? ` · ${t('черновик')}` : ''}
              </small>
            </button>
          ))}
        </aside>
        <section className="github-detail">
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
          {!creating && !selected && (
            <EmptyState compact icon={GitPullRequest} title={t('Выберите issue или pull request')} />
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
              {selected.kind === 'pull' && (
                <p className="github-meta">
                  {selected.detail.headRefName} → {selected.detail.baseRefName}
                  {selected.detail.reviewDecision ? ` · ${selected.detail.reviewDecision}` : ''}
                </p>
              )}
              {selected.detail.body?.trim() && <pre className="github-body">{selected.detail.body.trim()}</pre>}
              {selected.kind === 'issue' && (
                <div className="automation-actions">
                  <button
                    className="automation-primary"
                    disabled={busy}
                    onClick={() =>
                      void run(() =>
                        onAgentTask(`Issue #${selected.detail.number}`, issuePrompt(selected.detail as GithubIssueDetail))
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
                      setSelected({ ...selected, diff })
                    })
                  }
                  onReview={() =>
                    void run(() => onAgentTask(`Review PR #${selected.detail.number}`, reviewPrompt(selected.detail)))
                  }
                  onChecks={() =>
                    void run(async () => {
                      const logs = await api({ action: 'failedLogs', number: selected.detail.number })
                      await onAgentTask(`Checks PR #${selected.detail.number}`, checksPrompt(selected.detail, logs))
                    })
                  }
                />
              )}
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
  const feedback = detail.reviewComments.length + detail.reviews.filter((review) => review.body?.trim()).length
  return (
    <>
      <h4>{t('Проверки')}</h4>
      <ul className="github-checks">
        {detail.checks.map((check) => (
          <li key={`${check.workflow}-${check.name}`} className={`is-${check.bucket}`}>
            <span className="github-check-dot" />
            {check.workflow ? `${check.workflow} / ` : ''}
            {check.name}
          </li>
        ))}
        {!detail.checks.length && <li className="launch-placeholder">{t('Проверок нет')}</li>}
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
