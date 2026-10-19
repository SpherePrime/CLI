import { EmptyState } from '../../components/EmptyState'
import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowUpRight, Plus, RotateCcw, Square, Trash2, Users } from 'lucide-react'
import type { DesktopEvent } from '../../../../shared/contracts'
import {
  finishedTeamStatuses,
  type Team,
  type TeamApi,
  type TeamRequest,
  type TeamResponse,
  type TeamTask
} from '../../../../shared/teams'
import { useI18n } from '../../i18n'
import { TeamEditor } from './TeamEditor'
import { teamError, teamStatusLabel } from './presentation'
import '../../styles/automations.css'
import '../../styles/teams.css'

type Props = {
  root: string
  api: TeamApi
  subscribe(listener: (event: DesktopEvent) => void): () => void
  onError(message: string): void
  onOpenChat(run: { root: string; sessionId: string }): void
}

export function TeamPanel(props: Props) {
  return <TeamWorkspace key={props.root} {...props} />
}

function elapsed(task: TeamTask, now: number) {
  if (!task.startedAt) return ''
  const seconds = Math.max(0, Math.round(((task.finishedAt ?? now) - task.startedAt) / 1000))
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

function TeamWorkspace({ root, api, subscribe, onError, onOpenChat }: Props) {
  const { t } = useI18n()
  const [snapshot, setSnapshot] = useState<TeamResponse>({ root, teams: [] })
  const [editor, setEditor] = useState(false)
  const [busy, setBusy] = useState(false)
  const [now, setNow] = useState(Date.now())
  const mounted = useRef(true)
  const refresh = useCallback(async () => {
    try {
      const result = await api({ action: 'list' })
      if (mounted.current) setSnapshot(result)
    } catch (failure) {
      onError(teamError(failure, t))
    }
  }, [api, onError, t])
  useEffect(() => {
    mounted.current = true
    void refresh()
    const unsubscribe = subscribe((event) => {
      if (event.type === 'team' && event.payload?.root === root) void refresh()
    })
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => {
      mounted.current = false
      unsubscribe()
      clearInterval(timer)
    }
  }, [root, refresh, subscribe])
  const request = async (operation: TeamRequest) => {
    setBusy(true)
    try {
      const result = await api(operation)
      if (mounted.current) setSnapshot(result)
      return true
    } catch (failure) {
      onError(teamError(failure, t))
      return false
    } finally {
      if (mounted.current) setBusy(false)
    }
  }
  const progress = (team: Team) => {
    const done = team.tasks.filter((task) => finishedTeamStatuses.includes(task.status)).length
    return `${done}/${team.tasks.length}`
  }
  return (
    <div className="automation-panel team-panel" aria-label={t('Команды агентов')}>
      <header className="automation-header">
        <div>
          <Users size={17} aria-hidden="true" />
          <h2>{t('Команды агентов')}</h2>
        </div>
        <div className="automation-actions">
          {snapshot.teams.length > 0 && (
            <button className="automation-primary" onClick={() => setEditor(true)} disabled={busy || editor}>
              <Plus size={15} />
              {t('Новая группа')}
            </button>
          )}
        </div>
      </header>
      <div className="automation-content">
        {editor && (
          <TeamEditor
            busy={busy}
            onCancel={() => setEditor(false)}
            onSave={async (input) => {
              if (await request({ action: 'create', input })) setEditor(false)
            }}
          />
        )}
        {!snapshot.teams.length && !editor && (
          <EmptyState
            icon={Users}
            title={t('Параллельная работа нескольких агентов')}
            text={t('Разделите задачу между исполнителями. Каждый работает в своём чате.')}
            action={{ label: t('Новая группа'), icon: Plus, onClick: () => setEditor(true) }}
          />
        )}
        {snapshot.teams.map((team) => {
          const active = team.tasks.some((task) => !finishedTeamStatuses.includes(task.status))
          return (
            <article className="automation-rule team-card" key={team.id}>
              <div className="automation-rule-heading">
                <h3>{team.title}</h3>
                <span className="automation-status">{progress(team)}</span>
                <span className="team-meta">
                  {t('до {value0} одновременно', { value0: team.concurrency })}
                </span>
              </div>
              {team.goal && <p className="automation-rule-prompt">{team.goal}</p>}
              <ul className="team-tasks">
                {team.tasks.map((task) => (
                  <li key={task.id} className={`team-task is-${task.status}`}>
                    <span className="team-task-dot" aria-hidden="true" />
                    <div className="team-task-body">
                      <strong>{task.title}</strong>
                      <span>
                        {t(teamStatusLabel(task.status))}
                        {task.startedAt ? ` · ${elapsed(task, now)}` : ''}
                      </span>
                      {task.error && <p className="team-task-error">{task.error}</p>}
                    </div>
                    <div className="team-task-actions">
                      {task.sessionId && (
                        <button
                          aria-label={t('Открыть чат исполнителя {value0}', { value0: task.title })}
                          title={t('Открыть чат')}
                          onClick={() => onOpenChat({ root: team.root, sessionId: task.sessionId! })}
                        >
                          <ArrowUpRight size={14} />
                        </button>
                      )}
                      {!finishedTeamStatuses.includes(task.status) ? (
                        <button
                          aria-label={t('Остановить исполнителя {value0}', { value0: task.title })}
                          title={t('Остановить')}
                          disabled={busy}
                          onClick={() => void request({ action: 'stopTask', teamId: team.id, taskId: task.id })}
                        >
                          <Square size={13} />
                        </button>
                      ) : (
                        task.status !== 'completed' && (
                          <button
                            aria-label={t('Повторить исполнителя {value0}', { value0: task.title })}
                            title={t('Повторить')}
                            disabled={busy}
                            onClick={() => void request({ action: 'retryTask', teamId: team.id, taskId: task.id })}
                          >
                            <RotateCcw size={13} />
                          </button>
                        )
                      )}
                    </div>
                  </li>
                ))}
              </ul>
              <div className="automation-actions">
                {active ? (
                  <button disabled={busy} onClick={() => void request({ action: 'stopTeam', teamId: team.id })}>
                    <Square size={13} />
                    {t('Остановить группу')}
                  </button>
                ) : (
                  <button disabled={busy} onClick={() => void request({ action: 'remove', teamId: team.id })}>
                    <Trash2 size={13} />
                    {t('Удалить')}
                  </button>
                )}
              </div>
            </article>
          )
        })}
      </div>
    </div>
  )
}
