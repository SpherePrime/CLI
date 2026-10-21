import type { PrimeService } from '../prime/service'
import type { Operation, Project, Session } from '../../shared/contracts'
import type { RemoteProfile } from '../../shared/remote'
import { isRemoteWorkspace } from '../../shared/remoteWorkspace'

export type RemoteBackend = {
  profile: RemoteProfile
  service: PrimeService
  workspaceId: string
  key: string
  root?: string
}

export class WorkspaceRouter {
  constructor(
    readonly local: PrimeService,
    private readonly remotes: () => Iterable<RemoteBackend>
  ) {}
  get projects(): Map<string, Project> {
    const projects = new Map([...this.local.projects].filter(([, project]) => !project.remote))
    for (const entry of this.remotes()) {
      const project = entry.service.projects.get(entry.workspaceId)
      if (!project) continue
      projects.set(project.id, {
        ...project,
        path: entry.key,
        kind: 'project',
        remote: {
          profileId: entry.profile.id,
          name: entry.profile.name,
          host: entry.profile.host,
          port: entry.profile.port,
          user: entry.profile.user,
          root: entry.root ?? project.path
        }
      })
    }
    return projects
  }
  async open(path: string, channels: string[] = []): Promise<Project> {
    if (!isRemoteWorkspace(path)) return this.local.open(path, channels)
    const project = [...this.projects.values()].find((item) => item.path === path)
    if (!project)
      throw Object.assign(new Error('Подключитесь к SSH-проекту перед запуском задачи'), {
        code: 'REMOTE_DISCONNECTED'
      })
    return project
  }
  private owner(id: string): PrimeService {
    for (const entry of this.remotes())
      if (entry.workspaceId === id && entry.service.projects.has(id)) return entry.service
    if (this.local.projects.get(id)?.remote) throw new Error('Сервер отключён.')
    if (this.local.projects.has(id)) return this.local
    throw new Error('Проект не открыт')
  }
  async call(id: string, operation: Operation, args: any = {}): Promise<any> {
    return this.owner(id).call(id, operation, args)
  }
  projectSessions(paths: string[]): Promise<Record<string, Session[]>> {
    return this.local.projectSessions(paths)
  }
  root(id: string): string {
    const entry = [...this.remotes()].find((item) => item.workspaceId === id)
    return entry?.root ?? this.owner(id).root(id)
  }
  close(id: string): Promise<void> {
    return this.owner(id).close(id)
  }
  isBusy(): boolean {
    return this.local.isBusy() || [...this.remotes()].some((entry) => entry.service.isBusy())
  }
  async dispose(): Promise<void> {
    await Promise.all(
      [...new Set([this.local, ...[...this.remotes()].map((entry) => entry.service)])].map(
        (service) => service.dispose()
      )
    )
  }
}
