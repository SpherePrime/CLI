import { useCallback, useEffect, useState } from 'react'
import type { GithubRequest, GithubStatus } from '../../../../shared/github'
import { useI18n } from '../../i18n'
import { Select } from '../../components/Select'
import { githubErrorMessage } from './useGithubRepository'

export function CreatePull({
  status,
  api,
  onDone,
  onCancel,
  onError
}: {
  status: GithubStatus
  api(request: GithubRequest): Promise<any>
  onDone(url: string): void
  onCancel(): void
  onError(error: unknown): void
}) {
  const { t } = useI18n()
  const [branches, setBranches] = useState<string[]>([])
  const [head, setHead] = useState(status.branch ?? '')
  const [base, setBase] = useState(status.defaultBranch ?? 'main')
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [draft, setDraft] = useState(true)
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [branchError, setBranchError] = useState('')
  const [error, setError] = useState('')
  const loadBranches = useCallback(async () => {
    setLoading(true)
    setBranchError('')
    try {
      setBranches(await api({ action: 'branches' }))
    } catch (error) {
      setBranchError(githubErrorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [api])
  useEffect(() => {
    void loadBranches()
  }, [loadBranches])
  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      const result = await api({ action: 'createPull', head, base, title, body, draft })
      onDone(result.url)
    } catch (error) {
      setError(githubErrorMessage(error))
      onError(error)
    } finally {
      setBusy(false)
      setConfirm(false)
    }
  }
  return (
    <form
      className="automation-editor github-create"
      aria-busy={busy || loading}
      onSubmit={(event) => {
        event.preventDefault()
        if (busy || loading || !head || head === base || !title.trim()) return
        if (confirm) void submit()
        else setConfirm(true)
      }}
    >
      <h3>{t('Создать pull request')}</h3>
      {loading && <p role="status">{t('Загрузка…')}</p>}
      {branchError && (
        <div className="github-load-error" role="alert">
          <strong>{t('Не удалось загрузить ветки')}</strong>
          <p>{branchError}</p>
          <button type="button" onClick={() => void loadBranches()}>
            {t('Повторить')}
          </button>
        </div>
      )}
      <div className="github-create-branches">
        <label>
          {t('Ветка')}
          <Select
            label={t('Ветка')}
            value={head}
            onChange={(value) => {
              setHead(value)
              setConfirm(false)
            }}
            disabled={busy || loading}
            options={branches.map((branch) => ({ value: branch, label: branch }))}
          />
        </label>
        <label>
          {t('В ветку')}
          <Select
            label={t('В ветку')}
            value={base}
            onChange={(value) => {
              setBase(value)
              setConfirm(false)
            }}
            disabled={busy || loading}
            options={[
              ...new Set(
                [status.defaultBranch, ...branches].filter((branch): branch is string =>
                  Boolean(branch)
                )
              )
            ].map((branch) => ({ value: branch, label: branch }))}
          />
        </label>
      </div>
      <label>
        {t('Заголовок')}
        <input
          required
          maxLength={256}
          value={title}
          disabled={busy}
          onChange={(event) => {
            setTitle(event.target.value)
            setConfirm(false)
          }}
        />
      </label>
      <label>
        {t('Описание')}
        <textarea
          rows={4}
          maxLength={60000}
          disabled={busy}
          value={body}
          onChange={(event) => {
            setBody(event.target.value)
            setConfirm(false)
          }}
        />
      </label>
      <label className="github-checkbox">
        <input
          type="checkbox"
          checked={draft}
          disabled={busy}
          onChange={(event) => {
            setDraft(event.target.checked)
            setConfirm(false)
          }}
        />
        {t('Черновик')}
      </label>
      {!loading && head && head === base && (
        <p className="github-meta">
          {t('Для pull request выберите разные исходную и целевую ветки.')}
        </p>
      )}
      {error && (
        <p className="github-load-error" role="alert">
          {error}
        </p>
      )}
      {confirm && (
        <p className="github-confirm" role="alert">
          {t('Ветка {value0} будет отправлена на GitHub и откроется pull request в {value1}.', {
            value0: head,
            value1: base
          })}
        </p>
      )}
      <div className="automation-actions">
        <button type="button" onClick={onCancel} disabled={busy}>
          {t('Отмена')}
        </button>
        <button
          className="automation-primary"
          type="submit"
          disabled={
            busy || loading || Boolean(branchError) || !head || head === base || !title.trim()
          }
        >
          {t(confirm ? 'Подтвердить и опубликовать' : 'Создать pull request')}
        </button>
      </div>
    </form>
  )
}
