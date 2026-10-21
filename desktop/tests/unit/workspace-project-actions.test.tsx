import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, expect, it, vi } from 'vitest'
import { useWorkspace, type Workspace } from '../../src/renderer/src/app/useWorkspace'
import type { Project } from '../../src/shared/contracts'

afterEach(() => vi.unstubAllGlobals())

function workspace() {
  let value: Workspace
  function Probe() {
    value = useWorkspace()
    return null
  }
  renderToStaticMarkup(<Probe />)
  return value!
}

it('creates a new chat inside the selected project without opening personal chats', async () => {
  const project: Project = { id: 'project', path: '/work', clientId: 'client', connection: 'ready' }
  const created = { id: 'new-chat', title: 'Новый чат' }
  const call = vi.fn(async (id: string, operation: string) => {
    if (operation === 'createSession') return created
    if (operation === 'sessions' || operation === 'messages') return []
    if (operation === 'messagePage')
      return { messages: [], has_before: false, has_after: false, timing: { completed: {} } }
    return {}
  })
  const openChat = vi.fn(async () => ({ ...project, id: 'personal', kind: 'chat' }))
  vi.stubGlobal('window', { prime: { call, openChat, projects: async () => [project] } })
  const w = workspace()
  await w.selectProject(project)
  expect(await w.createSession()).toEqual(created)
  expect(call).toHaveBeenCalledWith('project', 'createSession', { title: 'Новый чат' })
  expect(openChat).not.toHaveBeenCalled()
})

it('reloads persisted project history after removing a project', async () => {
  const preferences = vi.fn(async () => ({ recent: [] }))
  const closeProject = vi.fn(async () => {})
  vi.stubGlobal('window', { prime: { preferences, closeProject } })
  await workspace().closeProject('removed')
  expect(closeProject).toHaveBeenCalledWith('removed')
  expect(preferences).toHaveBeenCalledOnce()
})
