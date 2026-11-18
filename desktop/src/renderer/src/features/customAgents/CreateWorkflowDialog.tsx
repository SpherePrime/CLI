import { useRef, useState } from 'react'
import { GitBranch, Save } from 'lucide-react'
import type {
  CreateWorkflowInput,
  CustomWorkflow,
  UpdateWorkflowPatch
} from '../../../../shared/customAgents'
import { Modal } from '../../components/Modal'
import { useI18n } from '../../i18n'

export function CreateWorkflowDialog({
  workflow,
  onCreate,
  onUpdate,
  close
}: {
  workflow?: CustomWorkflow | null
  onCreate: (input: CreateWorkflowInput) => Promise<CustomWorkflow>
  onUpdate: (workflowId: string, patch: UpdateWorkflowPatch) => Promise<CustomWorkflow>
  close: () => void
}) {
  const { t } = useI18n()
  const isEdit = Boolean(workflow)
  const [name, setName] = useState(workflow?.name || '')
  const [description, setDescription] = useState(workflow?.description || '')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const nameRef = useRef<HTMLInputElement>(null)

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) {
      setError(t('Укажите название цепочки'))
      return
    }
    setSubmitting(true)
    setError('')
    try {
      if (workflow) {
        await onUpdate(workflow.id, {
          name: trimmed,
          description: description.trim()
        })
      } else {
        await onCreate({
          name: trimmed,
          description: description.trim(),
          enabled: true
        })
      }
      close()
    } catch (cause) {
      setError(String((cause as Error)?.message || cause))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      title={isEdit ? t('Параметры цепочки') : t('Новая цепочка')}
      close={() => {
        if (!submitting) close()
      }}
      className="ca-workflow-modal"
      initialFocus={nameRef}
    >
      <form className="ca-form" onSubmit={handleSubmit}>
        <label className="ca-field">
          <span>{t('Название')}</span>
          <input
            ref={nameRef}
            type="text"
            value={name}
            onChange={(event) => {
              setName(event.target.value)
              if (error) setError('')
            }}
            placeholder={t('Например: Аудит релизов')}
          />
        </label>

        <label className="ca-field">
          <span>{t('Описание (необязательно)')}</span>
          <textarea
            rows={3}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder={t('Назначение цепочки и ожидаемый результат')}
          />
        </label>

        {error && (
          <p className="ca-error" role="alert">
            {error}
          </p>
        )}

        <div className="ca-modal-actions">
          <button type="button" onClick={close} disabled={submitting}>
            {t('Отмена')}
          </button>
          <button type="submit" className="primary" disabled={submitting}>
            {isEdit ? <Save size={14} aria-hidden="true" /> : <GitBranch size={14} aria-hidden="true" />}
            <span>
              {submitting
                ? t('Сохранение…')
                : isEdit
                  ? t('Сохранить')
                  : t('Создать цепочку')}
            </span>
          </button>
        </div>
      </form>
    </Modal>
  )
}
