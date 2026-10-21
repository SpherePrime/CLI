import { EmptyState } from '../../components/EmptyState'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  AppWindow,
  ArrowUpRight,
  Bot,
  Clock3,
  MessageCircleQuestion,
  Rocket,
  ShieldAlert,
  Square,
  TerminalSquare,
  Users
} from 'lucide-react'
import { Modal } from '../../components/Modal'
import { Select } from '../../components/Select'
import type { Workspace } from '../../app/useWorkspace'
import { activeJobStatuses, type Job, type JobKind } from '../../../../shared/jobs'
import type { Session } from '../../../../shared/contracts'
import { useI18n } from '../../i18n'
import '../../styles/jobs.css'

const icons: Record<JobKind, typeof Bot> = {
  agent: Bot,
  automation: Clock3,
  team: Users,
  terminal: TerminalSquare,
  app: Rocket
}
const kindLabels: Record<JobKind, string> = {
  agent: 'Агент',
  automation: 'Автоматизация',
  team: 'Команда',
  terminal: 'Терминал',
  app: 'Приложение'
}
const statusLabels: Record<Job['status'], string> = {
  queued: 'В очереди',
  running: 'Выполняется',
  waiting: 'Ждёт ответа',
  completed: 'Готово',
  failed: 'Ошибка',
  cancelled: 'Остановлено',
  interrupted: 'Прервано'
}
type Filter = 'all' | 'active' | 'attention' | 'failed' | 'done'

function duration(job: Job, now: number) {
  if (!job.startedAt) return ''
  const seconds = Math.max(0, Math.round(((job.finishedAt ?? now) - job.startedAt) / 1000))
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

export function JobsCenter({
  workspace: w,
  close,
  openTools
}: {
  workspace: Workspace
  close(): void
  openTools(view: 'terminal' | 'launch'): void
}) {
  const { t } = useI18n()
  const [jobs, setJobs] = useState<Job[]>([])
  const [filter, setFilter] = useState<Filter>('active')
  const [kind, setKind] = useState<JobKind | 'all'>('all')
  const [now, setNow] = useState(Date.now())
  const [busy, setBusy] = useState('')
  const refresh = useCallback(async () => {
    try {
      setJobs(await window.prime.jobs({ action: 'list' }))
    } catch (error) {
      w.errorHandler(error)
    }
  }, [w.errorHandler])
  useEffect(() => {
    void refresh()
    const timer = setInterval(() => {
      setNow(Date.now())
      void refresh()
    }, 2000)
    return () => clearInterval(timer)
  }, [refresh])
  const visible = useMemo(
    () =>
      jobs.filter((job) => {
        if (kind !== 'all' && job.kind !== kind) return false
        if (filter === 'active') return activeJobStatuses.includes(job.status)
        if (filter === 'attention') return Boolean(job.attention)
        if (filter === 'failed') return job.status === 'failed' || job.status === 'interrupted'
        if (filter === 'done') return !activeJobStatuses.includes(job.status)
        return true
      }),
    [jobs, filter, kind]
  )
  const counts = {
    active: jobs.filter((job) => activeJobStatuses.includes(job.status)).length,
    attention: jobs.filter((job) => job.attention).length,
    failed: jobs.filter((job) => job.status === 'failed' || job.status === 'interrupted').length
  }
  const open = async (job: Job) => {
    const project =
      w.projects.find((item) => item.id === job.workspaceId) ??
      w.projects.find((item) => item.path === job.projectKey)
    if (!project) {
      w.errorHandler(new Error(t('Проект задачи закрыт')))
      return
    }
    close()
    if (job.sessionId) {
      const sessions: Session[] = await window.prime.call(project.id, 'sessions')
      const session = sessions.find((item) => item.id === job.sessionId)
      if (session) await w.openSession(project, session, sessions)
      else if (w.project?.id !== project.id) await w.selectProject(project)
    } else {
      if (w.project?.id !== project.id) await w.selectProject(project)
      if (job.kind === 'terminal' || job.kind === 'app')
        openTools(job.kind === 'app' ? 'launch' : 'terminal')
    }
  }
  const stop = async (job: Job) => {
    setBusy(job.id)
    try {
      setJobs(await window.prime.jobs({ action: 'stop', id: job.id }))
    } catch (error) {
      w.errorHandler(error)
    } finally {
      setBusy('')
    }
  }
  const filters: [Filter, string, number?][] = [
    ['active', 'Активные', counts.active],
    ['attention', 'Ждут ответа', counts.attention],
    ['failed', 'Ошибки', counts.failed],
    ['done', 'Завершённые'],
    ['all', 'Все']
  ]
  return (
    <Modal title={t('Фоновые задачи')} close={close} wide className="jobs-modal">
      <div className="jobs-toolbar">
        <div className="jobs-filters" role="tablist">
          {filters.map(([id, label, count]) => (
            <button key={id} role="tab" aria-selected={filter === id} onClick={() => setFilter(id)}>
              {t(label)}
              {count ? <span>{count}</span> : null}
            </button>
          ))}
        </div>
        <Select
          label={t('Тип задачи')}
          value={kind}
          onChange={(value) => setKind(value as JobKind | 'all')}
          options={[
            { value: 'all', label: t('Все типы') },
            ...(Object.keys(kindLabels) as JobKind[]).map((item) => ({
              value: item,
              label: t(kindLabels[item])
            }))
          ]}
        />
      </div>
      <ul className="jobs-list">
        {visible.map((job) => {
          const Icon = icons[job.kind]
          return (
            <li key={job.id} className={`job is-${job.status}`}>
              <span className="job-icon">
                <Icon size={15} />
              </span>
              <div className="job-body">
                <strong>{job.title}</strong>
                <span>
                  {t(kindLabels[job.kind])} · {job.projectName}
                  {job.detail ? ` · ${job.detail}` : ''}
                </span>
                {job.attention && (
                  <span className="job-attention">
                    {job.attention === 'permission' ? (
                      <ShieldAlert size={12} />
                    ) : (
                      <MessageCircleQuestion size={12} />
                    )}
                    {t(job.attention === 'permission' ? 'Нужно разрешение' : 'Агент задал вопрос')}
                  </span>
                )}
                {job.error && <p className="job-error">{job.error}</p>}
              </div>
              <span className="job-status">{t(statusLabels[job.status])}</span>
              <span className="job-time">{duration(job, now)}</span>
              <div className="job-actions">
                <button
                  className="icon small"
                  aria-label={t('Открыть {value0}', { value0: job.title })}
                  title={t('Открыть')}
                  onClick={() => void open(job)}
                >
                  <ArrowUpRight size={14} />
                </button>
                {job.stoppable && (
                  <button
                    className="icon small"
                    aria-label={t('Остановить {value0}', { value0: job.title })}
                    title={t('Остановить')}
                    disabled={busy === job.id}
                    onClick={() => void stop(job)}
                  >
                    <Square size={13} />
                  </button>
                )}
              </div>
            </li>
          )
        })}
        {!visible.length && (
          <li className="jobs-empty">
            <EmptyState
              icon={AppWindow}
              title={t(filter === 'active' ? 'Сейчас ничего не выполняется' : 'Здесь пока пусто')}
              text={t(
                'Здесь появятся агенты, автоматизации, команды, терминалы и приложения всех проектов.'
              )}
            />
          </li>
        )}
      </ul>
    </Modal>
  )
}
