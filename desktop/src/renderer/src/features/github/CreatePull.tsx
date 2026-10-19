import { useEffect, useState } from 'react'
import type { GithubStatus } from '../../../../shared/github'
import { useI18n } from '../../i18n'
import { Select } from '../../components/Select'

export function CreatePull({
  status,
  api,
  onDone,
  onCancel,
  onError
}: {
  status: GithubStatus
  api(request: any): Promise<any>
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
  useEffect(() => {
    void api({ action: 'branches' }).then(setBranches).catch(onError)
  }, [api, onError])
  const submit = async () => {
    setBusy(true)
    try {
      const result = await api({ action: 'createPull', head, base, title, body, draft })
      onDone(result.url)
    } catch (error) {
      onError(error)
    } finally {
      setBusy(false)
      setConfirm(false)
    }
  }
  return (
    <form
      className="automation-editor github-create"
      onSubmit={(event) => {
        event.preventDefault()
        if (confirm) void submit()
        else setConfirm(true)
      }}
    >
      <div className="github-create-branches">
        <label>
          {t('Ветка')}
          <Select
            label={t('Ветка')}
            value={head}
            onChange={setHead}
            options={branches.map((branch) => ({ value: branch, label: branch }))}
          />
        </label>
        <label>
          {t('В ветку')}
          <Select
            label={t('В ветку')}
            value={base}
            onChange={setBase}
            options={branches.map((branch) => ({ value: branch, label: branch }))}
          />
        </label>
      </div>
      <label>
        {t('Заголовок')}
        <input
          required
          maxLength={256}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
      </label>
      <label>
        {t('Описание')}
        <textarea rows={4} value={body} onChange={(event) => setBody(event.target.value)} />
      </label>
      <label className="github-checkbox">
        <input
          type="checkbox"
          checked={draft}
          onChange={(event) => setDraft(event.target.checked)}
        />
        {t('Черновик')}
      </label>
      {confirm && (
        <p className="github-confirm" role="alert">
          {t('Ветка {value0} будет отправлена на GitHub и откроется pull request в {value1}.', {
            value0: head,
            value1: base
          })}
        </p>
      )}
      <div className="automation-actions">
        <button type="button" onClick={onCancel}>
          {t('Отмена')}
        </button>
        <button
          className="automation-primary"
          type="submit"
          disabled={busy || !head || head === base || !title.trim()}
        >
          {t(confirm ? 'Подтвердить и опубликовать' : 'Создать pull request')}
        </button>
      </div>
    </form>
  )
}
