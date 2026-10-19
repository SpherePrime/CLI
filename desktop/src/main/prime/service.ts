import { randomUUID } from 'node:crypto'
import { realpath, stat } from 'node:fs/promises'
import type { DesktopEvent, Operation, Project } from '../../shared/contracts'
import { PrimeTransport } from './transport'
import { operationRoute, redact } from './operations'
import { ActivityTracker } from './activity'

type ProjectEnvironment = { resolveRoot?: (path: string) => Promise<string>; caseSensitive?: boolean }

export class PrimeService {
  projects = new Map<string, Project>()
  private subscriptions = new Map<string, () => void>()
  private activity = new ActivityTracker()
  private shells = new Map<string, AbortController>()
  private opening = new Map<string, Promise<Project>>()
  private channels = new Map<string, any[]>()
  private initializing = new Map<string, Promise<any>>()
  constructor(
    private transport: PrimeTransport,
    private emit: (event: DesktopEvent) => void,
    private environment: ProjectEnvironment = {}
  ) {}
  isBusy(): boolean {
    return this.activity.isBusy() || this.shells.size > 0 || this.initializing.size > 0
  }
  async open(path: string, channels: string[] = []): Promise<Project> {
    const canonical = await (this.environment.resolveRoot ? this.environment.resolveRoot(path) : realpath(path))
    const key = this.pathKey(canonical)
    const pending = this.opening.get(key)
    if (pending) return pending
    const operation = this.openCanonical(canonical, channels)
    this.opening.set(key, operation)
    try {
      return await operation
    } finally {
      this.opening.delete(key)
    }
  }
  private async openCanonical(canonical: string, channels: string[]): Promise<Project> {
    if (!this.environment.resolveRoot && !(await stat(canonical)).isDirectory()) throw new Error('Выберите папку проекта')
    const existing = [...this.projects.values()].find(
      (project) => this.pathKey(project.path) === this.pathKey(canonical)
    )
    if (existing) return existing
    const clientId = randomUUID()
    const workspace: any = await this.transport.request('POST', '/v1/workspaces', {
      path: canonical,
      client_id: clientId,
      channels
    })
    const project: Project = {
      id: workspace.id,
      path: canonical,
      clientId,
      channels,
      connection: 'starting'
    }
    this.projects.set(project.id, project)
    let initialReady: () => void = () => {}
    const ready = new Promise<void>((resolve) => {
      initialReady = resolve
    })
    this.subscriptions.set(
      project.id,
      this.transport.subscribe(
        `/v1/workspaces/${project.id}/events?client_id=${clientId}`,
        (envelope) => {
          const payload = envelope.payload?.payload ?? envelope.payload
          this.activity.event(project.id, envelope.type, payload)
          if (envelope.type === 'mcp_channel_message')
            this.channels.set(
              project.id,
              [...(this.channels.get(project.id) || []), redact(payload)].slice(-200)
            )
          this.emit({
            workspaceId: project.id,
            type: envelope.type,
            payload: { action: envelope.payload?.type, data: redact(payload) }
          })
        },
        (connection) => {
          project.connection = connection
          this.emit({ workspaceId: project.id, type: 'connection', payload: connection })
          if (connection === 'ready') {
            initialReady()
            void this.transport
              .request<any[]>('GET', `/v1/workspaces/${project.id}/runs`)
              .then((records) => this.activity.reconcile(project.id, records || []))
              .catch(() => {})
          }
        }
      )
    )
    await Promise.race([
      ready,
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Не удалось открыть поток событий Prime')), 10000)
      )
    ])
    return project
  }
  async close(id: string): Promise<void> {
    const project = this.projects.get(id)
    if (!project) return
    await this.transport.request('DELETE', `/v1/clients/${project.clientId}`)
    this.subscriptions.get(id)?.()
    this.subscriptions.delete(id)
    this.projects.delete(id)
  }
  async call(id: string, operation: Operation, args: any = {}): Promise<any> {
    const project = this.projects.get(id)
    if (!project) throw new Error('Проект не открыт')
    if (operation === 'init') return this.initialize(id, args)
    if (operation === 'channelMessages') return this.channels.get(id) || []
    if (operation === 'send' && project.connection !== 'ready')
      throw new Error('Дождитесь восстановления соединения')
    if (operation === 'currentSession') args = { ...args, clientId: project.clientId }
    if (operation === 'currentSession') project.currentSessionId = args.sessionId
    if (operation === 'cancel') this.shells.get(`${id}:${args.sessionId}`)?.abort()
    const [method, route, body] = operationRoute(operation, args)
    if (operation === 'send') this.activity.accept(id, args.runId, args.sessionId)
    const shellKey = `${id}:${args.sessionId}`
    const controller = operation === 'shell' ? new AbortController() : undefined
    if (controller) this.shells.set(shellKey, controller)
    let response: any
    try {
      response = await this.transport.request(
        method,
        `/v1/workspaces/${encodeURIComponent(id)}/${route}`,
        body,
        operation === 'shell' ? 0 : 60000,
        controller?.signal
      )
    } catch (error) {
      if (operation !== 'send') throw error
      try {
        const record: any = await this.transport.request(
          'GET',
          `/v1/workspaces/${id}/runs/${encodeURIComponent(args.runId)}`
        )
        if (['completed', 'cancelled', 'failed', 'interrupted'].includes(record.status))
          this.activity.remove(id, args.runId)
        return record
      } catch {
        if ((error as any).status >= 400 && (error as any).status < 500)
          this.activity.forget(id, args.runId)
        throw error
      }
    } finally {
      if (controller) this.shells.delete(shellKey)
    }
    if (operation === 'sessions')
      void this.transport
        .request<any[]>('GET', `/v1/workspaces/${id}/runs`)
        .then((records) => this.activity.reconcile(id, records || []))
        .catch(() => {})
    return redact(response)
  }
  root(id: string): string {
    const project = this.projects.get(id)
    if (!project) throw new Error('Проект не открыт')
    return project.path
  }
  private pathKey(path: string) { return (this.environment.caseSensitive ?? process.platform !== 'win32') ? path : path.toLowerCase() }
  detach(): void {
    for (const project of this.projects.values()) this.emit({ workspaceId: project.id, type: 'connection', payload: 'disconnected' })
    for (const unsubscribe of this.subscriptions.values()) unsubscribe()
    for (const controller of this.shells.values()) controller.abort()
    this.subscriptions.clear()
    this.shells.clear()
    this.projects.clear()
    this.channels.clear()
    this.opening.clear()
    this.initializing.clear()
    this.activity = new ActivityTracker()
  }
  private async initialize(id: string, args: any): Promise<any> {
    const pending = this.initializing.get(id)
    if (pending) return pending
    const initialization = (async () => {
      const status = await this.call(id, 'agent')
      if (status?.is_ready) return status
      const [method, route, body] = operationRoute('init', args)
      return redact(
        await this.transport.request(
          method,
          `/v1/workspaces/${encodeURIComponent(id)}/${route}`,
          body
        )
      )
    })()
    this.initializing.set(id, initialization)
    try {
      return await initialization
    } finally {
      this.initializing.delete(id)
    }
  }
  async dispose(): Promise<void> {
    await Promise.allSettled(this.initializing.values())
    for (const id of [...this.projects.keys()]) await this.close(id).catch(() => {})
  }
}
