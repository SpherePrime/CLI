import { useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import type { TeamInput, TeamTaskInput } from '../../../../shared/teams'
import { useI18n } from '../../i18n'

const emptyTask = (): TeamTaskInput => ({ title: '', instruction: '' })

export function TeamEditor({
  busy,
  onSave,
  onCancel
}: {
  busy: boolean
  onSave(input: TeamInput): void
  onCancel(): void
}) {
  const { t } = useI18n()
  const [title, setTitle] = useState('')
  const [goal, setGoal] = useState('')
  const [concurrency, setConcurrency] = useState(2)
  const [tasks, setTasks] = useState<TeamTaskInput[]>([emptyTask(), emptyTask()])
  const update = (index: number, patch: Partial<TeamTaskInput>) =>
    setTasks((items) => items.map((item, position) => (position === index ? { ...item, ...patch } : item)))
  const valid = title.trim() && tasks.every((task) => task.title.trim() && task.instruction.trim())
  return (
    <form
      className="automation-editor team-editor"
      onSubmit={(event) => {
        event.preventDefault()
        if (valid) onSave({ title, goal, concurrency, tasks })
      }}
    >
      <label>
        {t('Название группы')}
        <input required maxLength={120} value={title} onChange={(event) => setTitle(event.target.value)} />
      </label>
      <label>
        {t('Общая цель')}
        <textarea rows={2} maxLength={8000} value={goal} onChange={(event) => setGoal(event.target.value)} />
      </label>
      <label>
        {t('Одновременно')}
        <select value={concurrency} onChange={(event) => setConcurrency(Number(event.target.value))}>
          {[1, 2, 3, 4, 5, 6].map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </select>
      </label>
      <div className="team-editor-tasks">
        {tasks.map((task, index) => (
          <fieldset key={index} className="team-editor-task">
            <legend>{t('Исполнитель {value0}', { value0: index + 1 })}</legend>
            <input
              required
              maxLength={120}
              placeholder={t('Задача')}
              aria-label={t('Задача исполнителя {value0}', { value0: index + 1 })}
              value={task.title}
              onChange={(event) => update(index, { title: event.target.value })}
            />
            <textarea
              required
              rows={3}
              maxLength={20000}
              placeholder={t('Инструкция')}
              aria-label={t('Инструкция исполнителя {value0}', { value0: index + 1 })}
              value={task.instruction}
              onChange={(event) => update(index, { instruction: event.target.value })}
            />
            {tasks.length > 1 && (
              <button
                type="button"
                aria-label={t('Убрать исполнителя')}
                onClick={() => setTasks((items) => items.filter((_, position) => position !== index))}
              >
                <Trash2 size={14} />
              </button>
            )}
          </fieldset>
        ))}
      </div>
      <div className="automation-actions">
        <button type="button" disabled={tasks.length >= 20} onClick={() => setTasks((items) => [...items, emptyTask()])}>
          <Plus size={14} />
          {t('Добавить исполнителя')}
        </button>
        <span className="team-spacer" />
        <button type="button" onClick={onCancel}>
          {t('Отмена')}
        </button>
        <button className="automation-primary" type="submit" disabled={busy || !valid}>
          {t('Запустить группу')}
        </button>
      </div>
    </form>
  )
}
