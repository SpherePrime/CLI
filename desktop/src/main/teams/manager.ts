import { randomUUID } from 'node:crypto'
import type { DesktopEvent } from '../../shared/contracts'
import type { AgentAdapter } from '../automations/manager'
import { AtomicStore } from '../persistence/atomicStore'
import {
  finishedTeamStatuses,
  type Team,
  type TeamInput,
  type TeamRequest,
  type TeamResponse,
  type TeamTask
} from '../../shared/teams'

type State = { version: 1; teams: Team[] }
type Options = {
  path: string
  adapter: AgentAdapter
  emit(event: DesktopEvent): void
  pollInterval?: number
}

const limits = { teams: 100, tasks: 20, concurrency: 6 }

function validate(input: TeamInput): TeamInput {
  const text = (value: unknown, max: number, required = true) => {
    if (typeof value !== 'string' || value.length > max || (required && !value.trim()))
      throw new Error('team.invalid')
    return value.trim()
  }
  if (!input || !Array.isArray(input.tasks) || !input.tasks.length || input.tasks.length > limits.tasks)
    throw new Error('team.invalid')
  const concurrency = Number(input.concurrency)
  if (!Number.isInteger(concurrency) || concurrency < 1 || concurrency > limits.concurrency)
    throw new Error('team.invalid')
  return {
    title: text(input.title, 120),
    goal: text(input.goal, 8000, false),
    concurrency,
    tasks: input.tasks.map((task) => ({
      title: text(task?.title, 120),
      instruction: text(task?.instruction, 20000)
    }))
  }
}

export function teamPrompt(team: Team, task: TeamTask): string {
  const others = team.tasks
    .filter((item) => item.id !== task.id)
    .map((item) => `- ${item.title}`)
    .join('\n')
  return [
    `You are one member of the agent team "${team.title}".`,
    team.goal ? `Team goal:\n${team.goal}` : '',
    `Your task: ${task.title}\n${task.instruction}`,
    others ? `Other members work in parallel on:\n${others}\nDo not do their work and avoid editing the same files.` : '',
    'Finish with a short summary of the result and changed files.'
  ]
    .filter(Boolean)
    .join('\n\n')
}

export class TeamManager {
  private state: State = { version: 1, teams: [] }
  private store: AtomicStore<State>
  private queue: Promise<unknown> = Promise.resolve()
  private timer?: ReturnType<typeof setInterval>
  private ticking = false
  private loaded = false
  private error?: string

  constructor(private options: Options) {
    this.store = new AtomicStore(options.path)
  }

  async load() {
    try {
      const saved = (await this.store.read()) as State | undefined
      if (saved) {
        if (saved.version !== 1 || !Array.isArray(saved.teams)) throw new Error()
        this.state = saved
        for (const team of this.state.teams)
          for (const task of team.tasks)
            if (task.status === 'starting') task.status = task.sessionId ? 'running' : 'queued'
      }
    } catch {
      this.error = 'team.corrupt'
    }
    this.loaded = true
  }

  start() {
    if (this.timer) return
    this.timer = setInterval(() => void this.tick(), this.options.pollInterval ?? 2000)
    void this.tick()
  }

  stop() {
    if (this.timer) clearInterval(this.timer)
    this.timer = undefined
  }

  all() {
    return structuredClone(this.state.teams)
  }

  activeCount() {
    return this.state.teams.reduce(
      (sum, team) => sum + team.tasks.filter((task) => !finishedTeamStatuses.includes(task.status) && task.status !== 'queued').length,
      0
    )
  }

  private serialize<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.queue.then(operation)
    this.queue = result.catch(() => {})
    return result
  }

  private async save(roots: string[]) {
    await this.store.write(this.state)
    for (const root of new Set(roots))
      this.options.emit({ workspaceId: '', type: 'team', payload: { root, active: this.activeCount() } })
  }

  private snapshot(root: string): TeamResponse {
    return structuredClone({
      root,
      teams: this.state.teams.filter((team) => team.root === root).sort((a, b) => b.createdAt - a.createdAt),
      ...(this.error ? { error: this.error } : {})
    })
  }

  private team(root: string, id: string) {
    const team = this.state.teams.find((item) => item.id === id && item.root === root)
    if (!team) throw new Error('team.missing')
    return team
  }

  handle(root: string, request: TeamRequest): Promise<TeamResponse> {
    if (request.action === 'list') return Promise.resolve(this.snapshot(root))
    if (!this.loaded) throw new Error('team.notLoaded')
    if (this.error) throw new Error(this.error)
    return this.serialize(async () => {
      switch (request.action) {
        case 'create': {
          const input = validate(request.input)
          if (this.state.teams.length >= limits.teams) {
            const removable = this.state.teams
              .filter((team) => team.tasks.every((task) => finishedTeamStatuses.includes(task.status)))
              .sort((a, b) => a.createdAt - b.createdAt)[0]
            if (!removable) throw new Error('team.limit')
            this.state.teams = this.state.teams.filter((team) => team !== removable)
          }
          this.state.teams.push({
            id: randomUUID(),
            root,
            title: input.title,
            goal: input.goal,
            concurrency: input.concurrency,
            createdAt: Date.now(),
            tasks: input.tasks.map((task) => ({ ...task, id: randomUUID(), status: 'queued' }))
          })
          break
        }
        case 'remove': {
          const team = this.team(root, request.teamId)
          if (team.tasks.some((task) => ['starting', 'running'].includes(task.status)))
            throw new Error('team.active')
          this.state.teams = this.state.teams.filter((item) => item !== team)
          break
        }
        case 'stopTeam': {
          const team = this.team(root, request.teamId)
          await Promise.all(team.tasks.map((task) => this.stopTask(team, task)))
          break
        }
        case 'stopTask': {
          const team = this.team(root, request.teamId)
          const task = team.tasks.find((item) => item.id === request.taskId)
          if (!task) throw new Error('team.missing')
          await this.stopTask(team, task)
          break
        }
        case 'retryTask': {
          const team = this.team(root, request.teamId)
          const task = team.tasks.find((item) => item.id === request.taskId)
          if (!task || !finishedTeamStatuses.includes(task.status)) throw new Error('team.missing')
          Object.assign(task, { status: 'queued' })
          delete task.error
          delete task.runId
          delete task.finishedAt
          break
        }
      }
      await this.save([root])
      void this.tick()
      return this.snapshot(root)
    })
  }

  private async stopTask(team: Team, task: TeamTask) {
    if (task.status === 'queued') {
      task.status = 'cancelled'
      task.finishedAt = Date.now()
      return
    }
    if (!['starting', 'running'].includes(task.status) || !task.runId) return
    try {
      await this.options.adapter.cancel(this.run(team, task))
      task.status = 'cancelled'
    } catch (error) {
      task.error = (error as Error).message
      return
    }
    task.finishedAt = Date.now()
  }

  private run(team: Team, task: TeamTask) {
    return {
      id: task.runId!,
      ruleId: team.id,
      root: team.root,
      title: task.title,
      trigger: 'manual' as const,
      status: 'running' as const,
      createdAt: task.startedAt ?? team.createdAt,
      workspaceId: task.workspaceId,
      sessionId: task.sessionId
    }
  }

  private async tick() {
    if (this.ticking || !this.loaded || this.error) return
    this.ticking = true
    try {
      await this.serialize(async () => {
        const changed: string[] = []
        for (const team of this.state.teams) {
          for (const task of team.tasks.filter((item) => item.status === 'running')) {
            try {
              const result = await this.options.adapter.status(this.run(team, task))
              if (result.status === 'running') continue
              task.status = result.status
              if (result.error) task.error = result.error
              task.finishedAt = Date.now()
              changed.push(team.root)
            } catch {}
          }
          const active = team.tasks.filter((item) => ['starting', 'running'].includes(item.status)).length
          const queued = team.tasks.filter((item) => item.status === 'queued').slice(0, Math.max(0, team.concurrency - active))
          for (const task of queued) {
            changed.push(team.root)
            task.status = 'starting'
            task.runId = randomUUID()
            task.startedAt = Date.now()
            await this.store.write(this.state)
            try {
              const prepared = await this.options.adapter.prepare({ root: team.root, title: `${team.title}: ${task.title}`, runId: task.runId })
              task.workspaceId = prepared.workspaceId
              task.sessionId = prepared.sessionId
              await this.options.adapter.dispatch({ root: team.root, ...prepared, runId: task.runId, prompt: teamPrompt(team, task) })
              task.status = 'running'
            } catch (error) {
              task.status = 'failed'
              task.error = (error as Error).message
              task.finishedAt = Date.now()
            }
          }
        }
        if (changed.length) await this.save(changed)
      })
    } finally {
      this.ticking = false
    }
  }

  async dispose() {
    this.stop()
    await this.queue.catch(() => {})
  }
}
