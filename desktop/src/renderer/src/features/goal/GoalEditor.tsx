import { useState } from 'react'
import { useI18n } from '../../i18n'

export function GoalEditor({
  initial,
  canSave,
  canStart,
  saving,
  onSave,
  onCancel
}: {
  initial: string
  canSave: boolean
  canStart: boolean
  saving: boolean
  onSave: (text: string, start: boolean) => void
  onCancel?: () => void
}) {
  const { t } = useI18n()
  const [text, setText] = useState(initial)
  const empty = !text.trim()
  return (
    <div className="goal-editor">
      <textarea
        autoFocus
        rows={5}
        value={text}
        placeholder={t('Что должно быть готово в итоге')}
        aria-label={t('Текст цели')}
        onChange={(event) => setText(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !empty && canStart)
            onSave(text, true)
        }}
      />
      <p className="goal-hint">
        {t('Агент уточнит детали, разобьёт цель на этапы и пройдёт их подряд с проверкой каждого.')}
      </p>
      <div className="goal-actions">
        {onCancel && (
          <button className="quiet" onClick={onCancel} disabled={saving}>
            {t('Отмена')}
          </button>
        )}
        {canSave && (
          <button className="quiet" onClick={() => onSave(text, false)} disabled={empty || saving}>
            {t('Сохранить')}
          </button>
        )}
        <button
          className="primary"
          onClick={() => onSave(text, true)}
          disabled={empty || saving || !canStart}
        >
          {t('Начать')}
        </button>
      </div>
    </div>
  )
}
