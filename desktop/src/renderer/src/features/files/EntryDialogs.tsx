import { useRef, useState } from 'react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'

export function EntryNameDialog({
  kind,
  parent,
  close,
  create
}: {
  kind: 'file' | 'folder'
  parent: string
  close: () => void
  create: (path: string) => Promise<void>
}) {
  const { t } = useI18n()
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  return (
    <Modal
      title={kind === 'file' ? t('Новый файл') : t('Новая папка')}
      close={close}
      initialFocus={input}
    >
      <form
        className="entry-dialog"
        onSubmit={(event) => {
          event.preventDefault()
          const value = name.trim().replace(/^[\\/]+|[\\/]+$/g, '')
          if (!value) return
          setSaving(true)
          setError('')
          void create(parent ? `${parent}/${value}` : value)
            .then(close)
            .catch((reason) => setError(reason?.message || String(reason)))
            .finally(() => setSaving(false))
        }}
      >
        <label className="form-label">
          {parent ? t('В папке {path}', { path: parent }) : t('В корне проекта')}
          <input
            ref={input}
            value={name}
            placeholder={kind === 'file' ? 'index.html' : 'src'}
            onChange={(event) => setName(event.target.value)}
            required
          />
        </label>
        {error && <p className="error-text">{error}</p>}
        <div className="approval-actions">
          <button type="button" className="quiet" onClick={close}>
            {t('Отмена')}
          </button>
          <button className="primary" disabled={saving || !name.trim()}>
            {t('Создать')}
          </button>
        </div>
      </form>
    </Modal>
  )
}

export function EntryDeleteDialog({
  path,
  directory,
  close,
  remove
}: {
  path: string
  directory: boolean
  close: () => void
  remove: () => Promise<void>
}) {
  const { t } = useI18n()
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  return (
    <Modal title={directory ? t('Удалить папку?') : t('Удалить файл?')} close={close}>
      <div className="entry-dialog">
        <p>{t('{path} будет перемещён в корзину.', { path })}</p>
        {error && <p className="error-text">{error}</p>}
        <div className="approval-actions">
          <button className="quiet" onClick={close}>
            {t('Отмена')}
          </button>
          <button
            className="danger"
            disabled={saving}
            onClick={() => {
              setSaving(true)
              void remove()
                .then(close)
                .catch((reason) => setError(reason?.message || String(reason)))
                .finally(() => setSaving(false))
            }}
          >
            {t('Удалить')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
