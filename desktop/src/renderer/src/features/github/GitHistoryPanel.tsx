import { useCallback, useEffect, useState } from 'react'
import { GitCommitHorizontal, RefreshCw, RotateCcw } from 'lucide-react'
import type { GitCommitDetail, GitHistory, GitRestoreResult } from '../../../../shared/gitHistory'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'
import '../../styles/git-history.css'

export function GitHistoryPanel({ workspaceId }: { workspaceId: string }) {
  const { t, locale } = useI18n()
  const [history, setHistory] = useState<GitHistory>()
  const [selected, setSelected] = useState<GitCommitDetail>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [confirm, setConfirm] = useState(false)
  const refresh = useCallback(async () => {
    setBusy(true)
    setError('')
    try {
      const next: GitHistory = await window.prime.github(workspaceId, { action: 'history' })
      setHistory(next)
      setSelected(undefined)
      if (next.commits[0])
        setSelected(
          await window.prime.github(workspaceId, { action: 'showCommit', sha: next.commits[0].sha })
        )
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : String(failure))
    } finally {
      setBusy(false)
    }
  }, [workspaceId])
  useEffect(() => {
    void refresh()
  }, [refresh])
  const open = async (sha: string) => {
    setBusy(true)
    setError('')
    setSelected(undefined)
    try {
      setSelected(await window.prime.github(workspaceId, { action: 'showCommit', sha }))
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : String(failure))
    } finally {
      setBusy(false)
    }
  }
  const restore = async () => {
    if (!selected) return
    setBusy(true)
    setError('')
    try {
      const result: GitRestoreResult = await window.prime.github(workspaceId, {
        action: 'restoreCommit',
        sha: selected.commit.sha,
        expectedHead: selected.currentHead
      })
      setConfirm(false)
      setNotice(
        t(
          result.changed
            ? 'Состояние восстановлено новым локальным коммитом. Для публикации выполните push.'
            : 'Проект уже соответствует этому состоянию.'
        )
      )
      if (result.backupStash)
        setNotice(
          (value) =>
            `${value} ${t('Локальные изменения сохранены в stash {value0}. Вернуть их: git stash apply --index {value0}', { value0: result.backupStash! })}`
        )
      await refresh()
    } catch (failure) {
      setConfirm(false)
      setError(failure instanceof Error ? failure.message : String(failure))
    } finally {
      setBusy(false)
    }
  }
  return (
    <div className="automation-panel github-panel git-history-panel">
      <header className="automation-header">
        <div>
          <GitCommitHorizontal size={17} />
          <h2>{t('История коммитов')}</h2>
          {history && <span className="automation-count">{history.branch}</span>}
        </div>
        <button aria-label={t('Обновить')} onClick={() => void refresh()} disabled={busy}>
          <RefreshCw size={14} className={busy ? 'spin' : ''} />
        </button>
      </header>
      {error && (
        <p className="git-history-message" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="git-history-message" role="status">
          {notice}
        </p>
      )}
      {history && !history.clean && (
        <p className="git-history-message">
          {t('Локальные изменения будут сохранены в резервную копию stash перед восстановлением.')}
        </p>
      )}
      <div className="git-history-layout">
        <aside className="git-history-list" aria-label={t('История коммитов')}>
          {!history?.commits.length && (
            <p className="launch-placeholder">{t(busy ? 'Загрузка…' : 'Коммитов нет')}</p>
          )}
          {history?.commits.map((commit) => (
            <button
              key={commit.sha}
              className={`git-history-item ${selected?.commit.sha === commit.sha ? 'selected' : ''}`}
              aria-pressed={selected?.commit.sha === commit.sha}
              onClick={() => void open(commit.sha)}
              disabled={busy}
            >
              <span className="git-history-item-top">
                <code>{commit.sha.slice(0, 8)}</code>
                {commit.sha === history.currentHead && (
                  <span className="git-history-head">HEAD</span>
                )}
                <time dateTime={commit.date}>
                  {new Date(commit.date).toLocaleDateString(locale)}
                </time>
              </span>
              <span className="git-history-title">{commit.title}</span>
              <span className="git-history-author">{commit.author}</span>
              {commit.refs && <span className="git-history-refs">{commit.refs}</span>}
            </button>
          ))}
        </aside>
        <section className="git-history-detail">
          {!selected && (
            <p className="launch-placeholder">{t('Выберите коммит для просмотра изменений')}</p>
          )}
          {selected && (
            <article className="git-history-card">
              <h3>{selected.commit.title}</h3>
              <p className="git-history-sha">{selected.commit.sha}</p>
              <p className="git-history-meta">
                {selected.commit.author} · {new Date(selected.commit.date).toLocaleString(locale)}
              </p>
              <div className="automation-actions">
                <button
                  className="automation-primary"
                  onClick={() => setConfirm(true)}
                  disabled={
                    busy ||
                    (Boolean(history?.clean) && selected.commit.sha === selected.currentHead)
                  }
                  title={
                    selected.commit.sha === selected.currentHead && history?.clean
                      ? t('Проект уже соответствует этому состоянию.')
                      : undefined
                  }
                >
                  <RotateCcw size={14} />
                  {t('Вернуть это состояние')}
                </button>
              </div>
              <pre className="git-history-diff">
                {selected.diff || t('Изменений в файлах проекта нет')}
              </pre>
              {selected.truncated && (
                <p className="git-history-meta">
                  {t('Показана часть изменений: diff слишком большой.')}
                </p>
              )}
            </article>
          )}
        </section>
      </div>
      {confirm && selected && (
        <Modal
          title={t('Вернуть это состояние')}
          close={() => {
            if (!busy) setConfirm(false)
          }}
        >
          <p>
            {t(
              'Файлы проекта будут восстановлены из коммита {value0}. Локальные изменения и новые файлы сохранятся в stash. Будет создан новый локальный коммит; существующая история сохранится. Изменения не отправляются на GitHub автоматически.',
              { value0: selected.commit.sha.slice(0, 12) }
            )}
          </p>
          <div className="automation-actions">
            <button onClick={() => setConfirm(false)} disabled={busy}>
              {t('Отмена')}
            </button>
            <button className="automation-primary" onClick={() => void restore()} disabled={busy}>
              {t(busy ? 'Восстановление…' : 'Подтвердить восстановление')}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
