import { z } from 'zod'
import type { Project } from '../../shared/contracts'
import type { Job, JobRequest, JobStatus } from '../../shared/jobs'
import type { AutomationManager } from '../automations/manager'
import type { TeamManager } from '../teams/manager'
import type { TerminalManager } from '../terminal/manager'
import type { LaunchManager } from '../launch/manager'
import type { WorkspaceRouter } from '../workbench/workspaceRouter'

type Options = {
  router(): WorkspaceRouter | undefined
  automations(): AutomationManager | undefined
  teams(): TeamManager | undefined
  launches(): LaunchManager | undefined
  terminals: TerminalManager
  label(project: Project): string
}

const requestSchema = z.discriminatedUnion('action', [
  z.object({ action: z.literal('list') }),
  z.object({ action: z.literal('stop'), id: z.string().min(3).max(300) })
])

const runStatus = (status: string): JobStatus =>
  status === 'accepted' || status === 'queued'
    ? 'queued'
    : status === 'running'
      ? 'running'
      : (['completed', 'failed', 'cancelled', 'interrupted'].includes(status) ? status : 'running') as JobStatus

const time = (value?: string | number) => (value ? new Date(value).getTime() || undefined : undefined)
const day = 24 * 60 * 60 * 1000
const agentJobsCacheMs = 1500

export class JobCenter {
  private agentCache?: { key: string; at: number; lists: Promise<Job[][]> }

  constructor(private options: Options) {}

  private agentLists(projects: Project[]) {
    const key = projects.map((project) => project.id).join('|')
    const cached = this.agentCache
    if (cached && cached.key === key && Date.now() - cached.at < agentJobsCacheMs) return cached.lists
    const lists = Promise.all(projects.map((project) => this.agentJobs(project)))
    this.agentCache = { key, at: Date.now(), lists }
    return lists
  }

  private projects() {
    return [...(this.options.router()?.projects.values() ?? [])]
  }

  private projectByKey(key: string) {
    const normalize = (value: string) => (value.startsWith('prime-ssh://') ? value : value.replaceAll('\\', '/').toLowerCase())
    return this.projects().find((project) => normalize(project.path) === normalize(key))
  }

  private async agentJobs(project: Project): Promise<Job[]> {
    const router = this.options.router()
    if (!router) return []
    const [runs, sessions, pending] = await Promise.all([
      router.call(project.id, 'runs').catch(() => []),
      router.call(project.id, 'sessions').catch(() => []),
      router.call(project.id, 'pending').catch(() => ({}))
    ])
    const titles = new Map<string, string>((Array.isArray(sessions) ? sessions : []).map((item: any) => [item.id, item.title]))
    const questions = pending?.questions ?? (pending?.question ? [pending.question] : [])
    const attention = new Map<string, Job['attention']>()
    for (const item of pending?.permissions ?? []) if (item.session_id) attention.set(item.session_id, 'permission')
    for (const item of Array.isArray(questions) ? questions : []) if (item.session_id) attention.set(item.session_id, 'question')
    const list: any[] = Array.isArray(runs) ? runs : runs?.runs ?? []
    return list
      .filter((run) => !run.finished_at || Date.now() - (time(run.finished_at) ?? 0) < day)
      .map((run) => {
        const status = runStatus(run.status)
        const active = status === 'queued' || status === 'running'
        return {
          id: `agent:${project.id}:${run.session_id}:${run.run_id}`,
          kind: 'agent',
          title: titles.get(run.session_id) || 'Чат',
          projectKey: project.path,
          projectName: this.options.label(project),
          workspaceId: project.id,
          sessionId: run.session_id,
          status: active && attention.has(run.session_id) ? 'waiting' : status,
          attention: active ? attention.get(run.session_id) : undefined,
          startedAt: time(run.accepted_at),
          finishedAt: time(run.finished_at),
          error: run.error || undefined,
          stoppable: active
        } satisfies Job
      })
  }

  async list(): Promise<Job[]> {
    const jobs: Job[] = []
    const projects = this.projects()
    const agentLists = await this.agentLists(projects)
    const owned = new Set<string>([
      ...(this.options.automations()?.activity() ?? []).map((run) => run.id),
      ...(this.options.teams()?.all() ?? []).flatMap((team) => team.tasks.flatMap((task) => (task.runId ? [task.runId] : [])))
    ])
    for (const list of agentLists)
      for (const job of list) if (!owned.has(job.id.split(':').pop()!)) jobs.push(job)
    for (const run of this.options.automations()?.activity() ?? []) {
      if (run.finishedAt && Date.now() - run.finishedAt > day) continue
      const project = this.projectByKey(run.root)
      jobs.push({
        id: `automation:${run.root}:${run.id}`,
        kind: 'automation',
        title: run.title,
        projectKey: run.root,
        projectName: project ? this.options.label(project) : run.root.split(/[\\/]/).pop() || run.root,
        workspaceId: project?.id,
        sessionId: run.sessionId,
        status: run.status === 'claimed' ? 'queued' : run.status,
        startedAt: run.createdAt,
        finishedAt: run.finishedAt,
        error: run.error,
        stoppable: run.status === 'claimed' || run.status === 'running'
      })
    }
    for (const team of this.options.teams()?.all() ?? []) {
      const project = this.projectByKey(team.root)
      for (const task of team.tasks) {
        if (task.finishedAt && Date.now() - task.finishedAt > day) continue
        jobs.push({
          id: `team:${team.root}:${team.id}:${task.id}`,
          kind: 'team',
          title: `${team.title}: ${task.title}`,
          projectKey: team.root,
          projectName: project ? this.options.label(project) : team.root,
          workspaceId: project?.id,
          sessionId: task.sessionId,
          status: task.status === 'starting' ? 'running' : task.status,
          startedAt: task.startedAt,
          finishedAt: task.finishedAt,
          error: task.error,
          stoppable: !['completed', 'failed', 'cancelled', 'interrupted'].includes(task.status)
        })
      }
    }
    const launches = this.options.launches()?.all() ?? []
    const launchTerminals = new Set(launches.map((item) => item.terminalId))
    for (const app of launches) {
      const project = this.projects().find((item) => item.id === app.workspaceId)
      jobs.push({
        id: `app:${app.configId}`,
        kind: 'app',
        title: app.name,
        projectKey: app.key,
        projectName: project ? this.options.label(project) : app.key,
        workspaceId: app.workspaceId,
        status: app.status === 'running' ? 'running' : app.exitCode ? 'failed' : 'completed',
        startedAt: app.startedAt,
        detail: app.url,
        error: app.status === 'exited' && app.exitCode ? `exit ${app.exitCode}` : undefined,
        stoppable: app.status === 'running'
      })
    }
    for (const terminal of this.options.terminals.list()) {
      if (launchTerminals.has(terminal.id) || terminal.status === 'closed') continue
      const project = this.projects().find((item) => item.id === terminal.workspaceId)
      jobs.push({
        id: `terminal:${terminal.workspaceId}:${terminal.id}`,
        kind: 'terminal',
        title: terminal.title,
        projectKey: project?.path ?? '',
        projectName: project ? this.options.label(project) : '',
        workspaceId: terminal.workspaceId,
        status: terminal.status === 'running' ? 'running' : terminal.exitCode ? 'failed' : 'completed',
        startedAt: terminal.createdAt,
        detail: terminal.owner === 'agent' ? 'agent' : undefined,
        stoppable: terminal.status === 'running'
      })
    }
    return jobs.sort((a, b) => (b.startedAt ?? 0) - (a.startedAt ?? 0))
  }

  async stop(id: string) {
    this.agentCache = undefined
    const [kind, ...rest] = id.split(':')
    if (kind === 'agent') {
      const [workspaceId, sessionId] = rest
      await this.options.router()?.call(workspaceId, 'cancel', { sessionId })
      return
    }
    if (kind === 'automation') {
      const runId = rest.pop()!
      await this.options.automations()?.cancel(rest.join(':'), runId)
      return
    }
    if (kind === 'team') {
      const taskId = rest.pop()!
      const teamId = rest.pop()!
      await this.options.teams()?.handle(rest.join(':'), { action: 'stopTask', teamId, taskId })
      return
    }
    if (kind === 'app') {
      await this.options.launches()?.stopProcess(rest[0])
      return
    }
    if (kind === 'terminal') {
      await this.options.terminals.close(rest[1], rest[0])
      return
    }
    throw new Error('Неизвестная задача')
  }

  async handle(input: unknown) {
    const request = requestSchema.parse(input) as JobRequest
    if (request.action === 'stop') await this.stop(request.id)
    return this.list()
  }
}
