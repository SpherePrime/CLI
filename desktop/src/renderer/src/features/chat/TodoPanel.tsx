import { useState } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import { useI18n } from '../../i18n'

type Todo = { content: string; status: string; active_form?: string }

const ringRadius = 13
const ringLength = 2 * Math.PI * ringRadius

export function TodoPanel({ todos }: { todos: Todo[] }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const done = todos.filter((todo) => todo.status === 'completed').length
  const activeIndex = todos.findIndex((todo) => todo.status === 'in_progress')
  const active = activeIndex >= 0 ? todos[activeIndex] : undefined
  const finished = done === todos.length
  const ratio = todos.length ? done / todos.length : 0
  const caption = finished
    ? t('План выполнен')
    : active
      ? t('Задача {current} из {total}', { current: activeIndex + 1, total: todos.length })
      : t('План · {total} задач', { total: todos.length })
  const headline = finished
    ? t('Все задачи выполнены')
    : active?.active_form ||
      active?.content ||
      todos.find((todo) => todo.status !== 'completed')?.content
  return (
    <section className={`todo-panel ${open ? 'open' : ''} ${finished ? 'finished' : ''}`}>
      <button
        className="todo-panel-head"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="todo-ring" aria-hidden="true">
          <svg width="32" height="32" viewBox="0 0 32 32">
            <circle cx="16" cy="16" r={ringRadius} className="todo-ring-track" />
            <circle
              cx="16"
              cy="16"
              r={ringRadius}
              className="todo-ring-value"
              strokeDasharray={ringLength}
              strokeDashoffset={ringLength * (1 - ratio)}
            />
          </svg>
          <span>{finished ? <Check size={13} strokeWidth={3} /> : `${done}/${todos.length}`}</span>
        </span>
        <span className="todo-panel-text">
          <small>{caption}</small>
          <span className={active ? 'todo-panel-live' : ''}>{headline}</span>
        </span>
        <span className="todo-steps" aria-hidden="true">
          {todos.map((todo, index) => (
            <i key={index} className={todo.status} />
          ))}
        </span>
        <ChevronDown size={15} className="todo-panel-chevron" />
      </button>
      {open && (
        <ol className="todo-timeline">
          {todos.map((todo, index) => (
            <li key={index} className={todo.status}>
              <span className="todo-marker">
                {todo.status === 'completed' ? <Check size={11} strokeWidth={3} /> : index + 1}
              </span>
              <span className="todo-content">
                {todo.status === 'in_progress' ? todo.active_form || todo.content : todo.content}
              </span>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
