import { Select } from '../../components/Select'
import { useI18n } from '../../i18n'
import { useCallback, useState } from 'react'
import { ShieldQuestion, CircleHelp, Check, X } from 'lucide-react'
import type { Workspace } from '../../app/useWorkspace'
import type { AgentWorker } from '../../../../shared/agents'
import { useAgentPolling } from '../agents/useAgentPolling'
import { agentApprovalSessions } from '../agents/agentApprovalSessions'
import { realSessionId } from '../../../../shared/pendingSession'
export function Approvals({ workspace: w }: { workspace: Workspace }) {
  const sessionId = realSessionId(w.session)
  if (!w.project || !sessionId) return null
  return (
    <TeamApprovals
      key={`${w.project.id}:${sessionId}`}
      workspace={w}
      projectId={w.project.id}
      sessionId={sessionId}
    />
  )
}
function TeamApprovals({
  workspace: w,
  projectId,
  sessionId
}: {
  workspace: Workspace
  projectId: string
  sessionId: string
}) {
  const { t } = useI18n()
  const hasOtherPending = [...w.permissions, ...w.questions].some(
    (item) => item.session_id !== sessionId
  )
  const load = useCallback(async () => {
    if (!hasOtherPending) return [] as AgentWorker[]
    const result = await window.prime.call(projectId, 'agentWorkers', { sessionId })
    return (result.workers || []) as AgentWorker[]
  }, [projectId, sessionId, hasOtherPending])
  const roster = useAgentPolling(load, 3000, [] as AgentWorker[])
  const sessions = agentApprovalSessions(sessionId, roster.value)
  return (
    <div className="approvals" aria-live="polite">
      {hasOtherPending && roster.error && (
        <div className="approval-card" role="alert">
          <p>
            {t('Не удалось проверить запросы агентов')}: {roster.error}
          </p>
          <button onClick={roster.refresh}>{t('Повторить')}</button>
        </div>
      )}
      {w.permissions
        .filter((permission) => sessions.has(permission.session_id))
        .map((permission) => (
          <Permission
            key={permission.id}
            permission={permission}
            workspace={w}
            workerName={sessions.get(permission.session_id)}
          />
        ))}
      {w.questions
        .filter((question) => sessions.has(question.session_id))
        .map((question) => (
          <Question
            key={question.id}
            request={question}
            workspace={w}
            workerName={sessions.get(question.session_id)}
          />
        ))}
    </div>
  )
}
function Permission({
  permission,
  workspace: w,
  workerName
}: {
  permission: any
  workspace: Workspace
  workerName?: string
}) {
  const { t } = useI18n()
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
        <ShieldQuestion size={18} /> {t('Разрешить действие?')}
      </h3>
      {workerName && <p className="muted">{t('Агент: {name}', { name: workerName })}</p>}
      <p>{permission.description || permission.tool_name}</p>
      {permission.path && <code>{permission.path}</code>}
      <details>
        <summary>
          {t('Параметры ·')} {permission.tool_name}
        </summary>
        <pre>{JSON.stringify(permission.params, null, 2)}</pre>
      </details>
      <div className="approval-actions">
        <button disabled={pending} onClick={() => void respond('deny')}>
          <X size={14} /> {t('Запретить')}
        </button>
        <button disabled={pending} onClick={() => void respond('allow_session')}>
          {t('На сессию')}
        </button>
        <button className="primary" disabled={pending} onClick={() => void respond('allow')}>
          <Check size={14} /> {t('Разрешить')}
        </button>
      </div>
    </section>
  )
}
function Question({
  request,
  workspace: w,
  workerName
}: {
  request: any
  workspace: Workspace
  workerName?: string
}) {
  const { t } = useI18n()
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
            session_id: request.session_id,
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
        {request.confirm_title || t('Уточнение задачи')}
      </h3>
      {workerName && <p className="muted">{t('Агент: {name}', { name: workerName })}</p>}
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
            <Select
              label={question.question}
              value={
                answers[question.id]?.yes === undefined
                  ? ''
                  : answers[question.id].yes
                    ? 'yes'
                    : 'no'
              }
              onChange={(value) => update(question.id, { yes: value === 'yes' })}
              options={[
                { value: '', label: t('Выберите ответ') },
                { value: 'yes', label: t('Да') },
                { value: 'no', label: t('Нет') }
              ]}
            />
          )}
          <input
            placeholder={t('Свой ответ или дополнение')}
            aria-label={t('Ответ: {value0}', { value0: question.question })}
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
          {t('Отменить')}
        </button>
        <button className="primary" disabled={pending}>
          {t('Ответить')}
        </button>
      </div>
    </form>
  )
}
