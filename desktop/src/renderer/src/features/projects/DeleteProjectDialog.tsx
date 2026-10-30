import { useState } from 'react'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'

export function DeleteProjectDialog({
  close,
  remove
}: {
  close: () => void
  remove: () => Promise<void>
}) {
  const { t } = useI18n()
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  return (
    <Modal title={t('Удалить проект и все чаты?')} close={close}>
      <div className="entry-dialog">
        <p>{t('Проект, все его чаты и настройки будут удалены без возможности восстановления.')}</p>
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
