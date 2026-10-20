import type { PrimeService } from '../prime/service'
import type { DesktopEvent, Project } from '../../shared/contracts'
import type { RemoteProfile, RemoteSnapshot } from '../../shared/remote'
import { remoteWorkspaceKey } from '../../shared/remoteWorkspace'
import type { RemoteBackend } from '../workbench/workspaceRouter'
import { RemoteManager } from './manager'
import type { RemoteConnection } from './connection'
import type { RemoteCredentialStore } from './credentials'
import { prepareLocalSshWorkspace } from './localWorkspace'
import { migrateRemoteHistory } from './historyMigration'

export type RemoteWorkspace = RemoteBackend & { connection: RemoteConnection; root: string }
type Resources = { service?: PrimeService; workspace?: RemoteWorkspace }
type Options = {
  path: string
  credentials?: RemoteCredentialStore
  workspaceDirectory: string
  service(): PrimeService
  emit(event: DesktopEvent): void
  channels(key: string): string[]
  onDetached(workspaceId: string): Promise<void>
}

export class RemoteWorkspaces {
  readonly manager: RemoteManager
  private resources = new Map<string, Resources>()
  private opening = new Map<string, Promise<Project>>()
  private transitions = new Map<string, Promise<unknown>>()
  constructor(private options: Options) {
    this.manager = new RemoteManager({
      path: options.path,
      credentials: options.credentials,
      emit: (snapshot: RemoteSnapshot) =>
        options.emit({ workspaceId: '', type: 'remote', payload: snapshot }),
      onReady: (profile, connection) => this.attach(profile, connection),
      onDisconnect: (id) => this.detach(id),
      onLost: (id) => void this.detach(id)
    })
  }
  backends(): RemoteWorkspace[] {
    return [...this.resources.values()].flatMap((entry) =>
      entry.workspace ? [entry.workspace] : []
    )
  }
  byWorkspace(workspaceId: string) {
    return this.backends().find((entry) => entry.workspaceId === workspaceId)
  }
  byProfile(profileId: string) {
    return this.resources.get(profileId)?.workspace
  }
  isBusy() {
    return this.backends().some((entry) => entry.service.isBusy())
  }
  private transition<T>(profileId: string, operation: () => Promise<T>): Promise<T> {
    const previous = this.transitions.get(profileId) ?? Promise.resolve()
    const pending = previous.catch(() => {}).then(operation)
    this.transitions.set(profileId, pending)
    void pending
      .finally(() => {
        if (this.transitions.get(profileId) === pending) this.transitions.delete(profileId)
      })
      .catch(() => {})
    return pending
  }
  private attach(profile: RemoteProfile, connection: RemoteConnection) {
    return this.transition(profile.id, () => this.attachConnection(profile, connection))
  }
  private async attachConnection(profile: RemoteProfile, connection: RemoteConnection) {
    const resources: Resources = {}
    this.resources.set(profile.id, resources)
    try {
      const key = remoteWorkspaceKey(profile.id, connection.root)
      this.manager.setProgress(profile.id, 'remote.preparingLocalAgent')
      const localRoot = await prepareLocalSshWorkspace(
        this.options.workspaceDirectory,
        profile,
        connection.root
      )
      await migrateRemoteHistory(connection, localRoot)
      if (this.resources.get(profile.id) !== resources) return
      resources.service = this.options.service()
      const project = await resources.service.open(localRoot, this.options.channels(key), {
        profileId: profile.id,
        name: profile.name,
        host: profile.host,
        port: profile.port,
        user: profile.user,
        root: connection.root
      })
      resources.workspace = {
        profile,
        service: resources.service,
        workspaceId: project.id,
        key,
        connection,
        root: connection.root
      }
      if (this.resources.get(profile.id) === resources)
        this.options.emit({ workspaceId: project.id, type: 'projectsChanged', payload: {} })
    } finally {
      if (this.resources.get(profile.id) !== resources) await this.release(resources)
    }
  }
  private detach(profileId: string) {
    return this.transition(profileId, () => this.detachConnection(profileId))
  }
  private async detachConnection(profileId: string) {
    const resources = this.resources.get(profileId)
    if (!resources) return
    this.resources.delete(profileId)
    await this.release(resources)
  }
  private async release(resources: Resources) {
    const workspaceId = resources.workspace?.workspaceId
    if (workspaceId) {
      const sessions = await resources.service?.call(workspaceId, 'sessions').catch(() => [])
      await Promise.all(
        (Array.isArray(sessions) ? sessions : [])
          .filter((session) => session.is_busy)
          .map((session) =>
            resources.service
              ?.call(workspaceId, 'cancel', { sessionId: session.id })
              .catch(() => {})
          )
      )
      await this.options.onDetached(workspaceId).catch(() => {})
      await resources.service?.close(workspaceId).catch(() => {})
    }
    this.options.emit({ workspaceId: workspaceId ?? '', type: 'projectsChanged', payload: {} })
  }
  async project(profileId: string): Promise<Project> {
    const entry = this.byProfile(profileId)
    const project = entry?.service.projects.get(entry.workspaceId)
    if (!project) throw new Error('remote.notConnected')
    return project
  }
  open(profileId: string): Promise<Project> {
    const pending = this.opening.get(profileId)
    if (pending) return pending
    const operation = (async () => {
      if (!this.byProfile(profileId)) {
        const profile = this.manager.list().profiles.find((entry) => entry.id === profileId)
        if (profile?.status === 'connected')
          await this.attach(profile, this.manager.connection(profileId))
        else await this.manager.connect(profileId)
      }
      return this.project(profileId)
    })()
    this.opening.set(profileId, operation)
    void operation
      .finally(() => {
        if (this.opening.get(profileId) === operation) this.opening.delete(profileId)
      })
      .catch(() => {})
    return operation
  }
  shutdown() {
    return this.manager.shutdown()
  }
}
