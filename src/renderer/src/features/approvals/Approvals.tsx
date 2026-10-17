import { useState } from 'react'
import { ShieldQuestion, CircleHelp, Check, X } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
export function Approvals({ workspace: w }: { workspace: Workspace }) {
  return (
    <div className="approvals">
      {w.permissions
        .filter((permission) => permission.session_id === w.session?.id)
        .map((permission) => (
          <Permission key={permission.id} permission={permission} workspace={w} />
        ))}
      {w.questions
        .filter((question) => question.session_id === w.session?.id)
        .map((question) => (
          <Question key={question.id} request={question} workspace={w} />
        ))}
    </div>
  )
}
function Permission({ permission, workspace: w }: { permission: any; workspace: Workspace }) {
  const [pending, setPending] = useState(false)
  const [done, setDone] = useState(false)
  async function respond(action: string) {
    setPending(true)
    try {
      await w.call('permission', { permission, action })
      setDone(true)
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setPending(false)
    }
  }
  if (done) return null
  return (
    <section className="approval-card">
      <h3>
        <ShieldQuestion size={18} /> Разрешить действие?
      </h3>
      <p>{permission.description || permission.tool_name}</p>
      {permission.path && <code>{permission.path}</code>}
      <details>
        <summary>Параметры · {permission.tool_name}</summary>
        <pre>{JSON.stringify(permission.params, null, 2)}</pre>
      </details>
      <div className="approval-actions">
        <button disabled={pending} onClick={() => void respond('deny')}>
          <X size={14} /> Запретить
        </button>
        <button disabled={pending} onClick={() => void respond('allow_session')}>
          На сессию
        </button>
        <button className="primary" disabled={pending} onClick={() => void respond('allow')}>
          <Check size={14} /> Разрешить
        </button>
      </div>
    </section>
  )
}
function Question({ request, workspace: w }: { request: any; workspace: Workspace }) {
  const [answers, setAnswers] = useState<Record<string, any>>({})
  const [pending, setPending] = useState(false)
  const [done, setDone] = useState(false)
  if (done) return null
  const update = (id: string, value: any) =>
    setAnswers((items) => ({ ...items, [id]: { ...items[id], ...value } }))
  return (
    <form
      className="approval-card"
      onSubmit={async (event) => {
        event.preventDefault()
        setPending(true)
        try {
          await w.call('answer', {
            batch_request_id: request.id,
            responses: request.questions.map((question: any) => ({
              request_id: question.id,
              ...answers[question.id]
            }))
          })
          setDone(true)
        } catch (error) {
          w.errorHandler(error)
        } finally {
          setPending(false)
        }
      }}
    >
      <h3>
        <CircleHelp size={18} />
        {request.confirm_title || 'Уточнение задачи'}
      </h3>
      {request.questions.map((question: any) => (
        <fieldset key={question.id}>
          <legend>{question.question}</legend>
          {question.description && <p>{question.description}</p>}
          {question.choices?.map((choice: any) => (
            <label className="choice" key={choice.id}>
              <input
                type={question.type?.includes('multi') ? 'checkbox' : 'radio'}
                name={question.id}
                value={choice.id}
                onChange={(event) =>
                  update(question.id, {
                    selected_ids: question.type?.includes('multi')
                      ? event.target.checked
                        ? [...(answers[question.id]?.selected_ids || []), choice.id]
                        : (answers[question.id]?.selected_ids || []).filter(
                            (id: string) => id !== choice.id
                          )
                      : [choice.id]
                  })
                }
              />
              <span>
                {choice.label}
                <small>{choice.description}</small>
              </span>
            </label>
          ))}
          {question.type === 'yes_no' && (
            <select
              aria-label={question.question}
              onChange={(event) => update(question.id, { yes: event.target.value === 'yes' })}
            >
              <option value="">Выберите ответ</option>
              <option value="yes">Да</option>
              <option value="no">Нет</option>
            </select>
          )}
          <input
            placeholder="Свой ответ или дополнение"
            aria-label={`Ответ: ${question.question}`}
            value={answers[question.id]?.fill_in_text || ''}
            onChange={(event) => update(question.id, { fill_in_text: event.target.value })}
          />
        </fieldset>
      ))}
      <div className="approval-actions">
        <button
          type="button"
          disabled={pending}
          onClick={() =>
            void w
              .call('cancelQuestion', { id: request.id })
              .then(() => setDone(true))
              .catch(w.errorHandler)
          }
        >
          Отменить
        </button>
        <button className="primary" disabled={pending}>
          Ответить
        </button>
      </div>
    </form>
  )
}
