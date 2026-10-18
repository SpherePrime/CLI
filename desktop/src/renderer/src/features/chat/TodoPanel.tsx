import { useState } from 'react'
import { Check, ChevronDown, Circle, LoaderCircle } from 'lucide-react'
import { useI18n } from '../../i18n'

type Todo = { content: string; status: string; active_form?: string }

export function TodoPanel({ todos }: { todos: Todo[] }) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const done = todos.filter((todo) => todo.status === 'completed').length
  const active = todos.find((todo) => todo.status === 'in_progress')
  const finished = done === todos.length
  const progress = todos.length ? (done / todos.length) * 100 : 0
  const headline = finished
    ? t('Все задачи выполнены')
    : active?.active_form || active?.content || t('План готов')
  return (
    <section className={`todo-panel ${open ? 'open' : ''} ${finished ? 'finished' : ''}`}>
      <button
        className="todo-panel-head"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="todo-panel-count">
          {finished ? <Check size={13} /> : `${done}/${todos.length}`}
        </span>
        <span className="todo-panel-headline">{headline}</span>
        <ChevronDown size={14} className="todo-panel-chevron" />
      </button>
      <div className="todo-panel-bar" aria-hidden="true">
        <span style={{ width: `${progress}%` }} />
      </div>
      {open && (
        <ol className="todo-panel-list">
          {todos.map((todo, index) => (
            <li key={index} className={todo.status}>
              {todo.status === 'completed' ? (
                <Check size={13} />
              ) : todo.status === 'in_progress' ? (
                <LoaderCircle size={13} className="spin" />
              ) : (
                <Circle size={13} />
              )}
              <span>{todo.content}</span>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
