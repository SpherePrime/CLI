import type { AgentAdapter } from '../automations/manager'
import type { PrimeService } from '../prime/service'
import type { Project } from '../../shared/contracts'

type Options = {
  service: () => Pick<PrimeService, 'projects' | 'open' | 'call'>
  channels?: (root: string) => string[]
  onProject?: (project: Project) => void
  filesFingerprint?: (root: string) => Promise<string>
}

const projectKey = (root: string) =>
  process.platform === 'win32' ? root.replaceAll('\\', '/').toLowerCase() : root

export function createAgentAdapter(options: Options): AgentAdapter {
  const project = async (root: string) => {
    const service = options.service()
    const existing = [...service.projects.values()].find(
      (item) => projectKey(item.path) === projectKey(root)
    )
    if (existing) return { service, workspace: existing }
    const workspace = await service.open(root, options.channels?.(root) || [])
    workspace.kind ||= 'project'
    options.onProject?.(workspace)
    return { service, workspace }
  }
  return {
    async prepare(input) {
      const { service, workspace } = await project(input.root)
      await service.call(workspace.id, 'init')
      const session = await service.call(workspace.id, 'createSession', { title: input.title })
      if (!session?.id) throw new Error('Не удалось создать чат для задачи')
      return { workspaceId: workspace.id, sessionId: session.id }
    },
    async dispatch(input) {
      const { service, workspace } = await project(input.root)
      await service.call(workspace.id, 'send', {
        sessionId: input.sessionId,
        runId: input.runId,
        prompt: input.prompt
      })
    },
    async status(run) {
      try {
        const { service, workspace } = await project(run.root)
        const record = await service.call(workspace.id, 'runs', { runId: run.id })
        if (['accepted', 'queued', 'running'].includes(record?.status))
          return { status: 'running', ...(record.error ? { error: record.error } : {}) }
        if (['completed', 'failed', 'cancelled', 'interrupted'].includes(record?.status))
          return { status: record.status, ...(record.error ? { error: record.error } : {}) }
        throw new Error('Prime вернул неизвестное состояние задачи')
      } catch (error) {
        if (
          (error as { status?: number }).status === 404 ||
          ['ENOENT', 'ENOTDIR'].includes((error as NodeJS.ErrnoException).code || '')
        )
          return { status: 'interrupted', error: 'Запуск не найден после восстановления' }
        throw error
      }
    },
    async cancel(run) {
      if (!run.sessionId) throw new Error('У задачи ещё нет чата')
      const { service, workspace } = await project(run.root)
      await service.call(workspace.id, 'cancel', { sessionId: run.sessionId })
      const deadline = Date.now() + 15000
      while (true) {
        const record = await service.call(workspace.id, 'runs', { runId: run.id })
        if (['completed', 'failed', 'cancelled', 'interrupted'].includes(record?.status)) return
        if (Date.now() >= deadline)
          throw new Error('Prime ещё останавливает задачу. Проверьте её состояние.')
        await new Promise((resolve) => setTimeout(resolve, 100))
      }
    },
    filesFingerprint: options.filesFingerprint
  }
}
