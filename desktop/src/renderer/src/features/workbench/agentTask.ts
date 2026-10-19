import type { Workspace } from '../../app/useWorkspace'
import type { Session } from '../../../../shared/contracts'

export async function startAgentTask(w: Workspace, title: string, prompt: string) {
  if (!w.project) throw new Error('Проект не открыт')
  const session: Session = await window.prime.call(w.project.id, 'createSession', { title: title.slice(0, 80) })
  await w.selectSession(session)
  await w.send(prompt)
}
