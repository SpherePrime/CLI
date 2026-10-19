import { PrimeService } from '../prime/service'
import type { DesktopEvent, Project } from '../../shared/contracts'
import type { RemoteProfile, RemoteSnapshot } from '../../shared/remote'
import { remoteWorkspaceKey } from '../../shared/remoteWorkspace'
import type { RemoteBackend } from '../workbench/workspaceRouter'
import { RemoteManager } from './manager'
import type { RemoteConnection } from './connection'
import { RemoteRuntime } from '../remoteRuntime/session'
import { createRemoteForward } from '../remoteRuntime/forward'
import type { BrowserMcpServer } from '../browser/mcp-server'

export type RemoteWorkspace = RemoteBackend & { connection: RemoteConnection }

type Resources = {
  runtime?: RemoteRuntime
  service?: PrimeService
  browser?: BrowserMcpServer
  forward?: { port: number; close(): Promise<void> }
  workspace?: RemoteWorkspace
}

type Options = {
  path: string
  runtimeDir: string
  emit(event: DesktopEvent): void
  channels(key: string): string[]
  startBrowser?(resolveWorkspace: (root: string) => string | undefined): Promise<BrowserMcpServer>
  onDetached(workspaceId: string): Promise<void>
}

export class RemoteWorkspaces {
  readonly manager: RemoteManager
  private resources = new Map<string, Resources>()

  constructor(private options: Options) {
    this.manager = new RemoteManager({
      path: options.path,
      emit: (snapshot: RemoteSnapshot) => options.emit({ workspaceId: '', type: 'remote', payload: snapshot }),
      onReady: (profile, connection) => this.attach(profile, connection),
      onDisconnect: (id) => this.detach(id),
      onLost: (id) => void this.detach(id)
    })
  }

  backends(): RemoteWorkspace[] {
    return [...this.resources.values()].flatMap((entry) => (entry.workspace ? [entry.workspace] : []))
  }

  byWorkspace(workspaceId: string): RemoteWorkspace | undefined {
    return this.backends().find((entry) => entry.workspaceId === workspaceId)
  }

  byProfile(profileId: string): RemoteWorkspace | undefined {
    return this.resources.get(profileId)?.workspace
  }

  isBusy(): boolean {
    return this.backends().some((entry) => entry.service.isBusy())
  }

  private async attach(profile: RemoteProfile, connection: RemoteConnection) {
    const resources: Resources = {}
    this.resources.set(profile.id, resources)
    try {
      await this.prepare(profile, connection, resources)
    } finally {
      if (this.resources.get(profile.id) !== resources) await this.release(resources)
    }
  }

  private async prepare(profile: RemoteProfile, connection: RemoteConnection, resources: Resources) {
    const key = remoteWorkspaceKey(profile.id, connection.root)
    const environment: Record<string, string> = {}
    if (this.options.startBrowser) {
      try {
        resources.browser = await this.options.startBrowser((root) =>
          root === connection.root ? resources.workspace?.workspaceId : undefined
        )
        const port = Number(new URL(resources.browser.url).port)
        resources.forward = await createRemoteForward(connection.client, port)
        environment.PRIME_DESKTOP_BROWSER_URL = `http://127.0.0.1:${resources.forward.port}/mcp`
        environment.PRIME_DESKTOP_BROWSER_TOKEN = resources.browser.token
      } catch {
        await resources.forward?.close().catch(() => {})
        await resources.browser?.close().catch(() => {})
        resources.forward = undefined
        resources.browser = undefined
      }
    }
    resources.runtime = await RemoteRuntime.start(profile, connection, {
      runtimeDir: this.options.runtimeDir,
      environment,
      progress: (message, percent) => this.manager.setProgress(profile.id, message, percent),
      onExit: () => void this.manager.fail(profile.id, 'remote.runtimeFailed')
    })
    resources.service = new PrimeService(resources.runtime.transport, this.options.emit, {
      resolveRoot: async () => connection.root,
      caseSensitive: true
    })
    const project = await resources.service.open(connection.root, this.options.channels(key))
    if (this.resources.get(profile.id) !== resources) return
    resources.workspace = { profile, service: resources.service, workspaceId: project.id, key, connection }
    this.options.emit({ workspaceId: project.id, type: 'projectsChanged', payload: {} })
  }

  private async detach(profileId: string) {
    const resources = this.resources.get(profileId)
    if (!resources) return
    this.resources.delete(profileId)
    await this.release(resources)
  }

  private async release(resources: Resources) {
    const workspaceId = resources.workspace?.workspaceId
    resources.service?.detach()
    if (workspaceId) await this.options.onDetached(workspaceId).catch(() => {})
    await resources.runtime?.close().catch(() => {})
    await resources.forward?.close().catch(() => {})
    await resources.browser?.close().catch(() => {})
    this.options.emit({ workspaceId: workspaceId ?? '', type: 'projectsChanged', payload: {} })
  }

  async project(profileId: string): Promise<Project> {
    const entry = this.byProfile(profileId)
    if (!entry) throw new Error('remote.notConnected')
    const project = entry.service.projects.get(entry.workspaceId)
    if (!project) throw new Error('remote.notConnected')
    return project
  }

  shutdown() {
    return this.manager.shutdown()
  }
}
