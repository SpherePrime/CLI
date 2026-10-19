import type { Workspace } from '../../app/useWorkspace'
import type { Session } from '../../../../shared/contracts'
import { isRemoteWorkspace } from '../../../../shared/remoteWorkspace'

export async function openWorkbenchRun(
  workspace: Workspace,
  run: { root: string; sessionId: string }
) {
  const key = (root: string) => isRemoteWorkspace(root) ? root : root.replaceAll('\\', '/').toLowerCase()
  const projects = await window.prime.projects()
  const project = projects.find((item) => key(item.path) === key(run.root))
  if (!project) throw new Error('Проект задачи недоступен')
  await workspace.selectProject(project)
  const sessions: Session[] = await window.prime.call(project.id, 'sessions')
  const session = sessions.find((item) => item.id === run.sessionId)
  if (!session) throw new Error('Чат задачи недоступен')
  await workspace.selectSession(session)
}
