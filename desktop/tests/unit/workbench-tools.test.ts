import { afterEach, expect, it } from 'vitest'
import { startBrowserMcpServer, type BrowserMcpServer } from '../../src/main/browser/mcp-server'
import type { BrowserPanel } from '../../src/main/browser/panel'
import { TerminalManager } from '../../src/main/terminal/manager'
import { workbenchTerminalTools } from '../../src/main/workbench/tools'
import { terminalOperation } from '../../src/main/workbench/terminalIpc'

let server: BrowserMcpServer | undefined
const managers: TerminalManager[] = []
afterEach(async () => {
  await server?.close()
  await Promise.all(managers.map((manager) => manager.dispose()))
  managers.length = 0
})

it('exposes workbench tools only to an authenticated bound workspace', async () => {
  server = await startBrowserMcpServer({} as BrowserPanel, {
    resolveWorkspace: (path) => (path === 'C:/project' ? 'project-a' : undefined),
    tools: () => [
      {
        name: 'workspace_probe',
        description: 'test',
        inputSchema: { type: 'object' },
        run: async (_args, context) => [{ type: 'text', text: context.workspaceId || '' }]
      }
    ]
  })
  const call = async (root?: string, token = server!.token, method = 'tools/list') => {
    const response = await fetch(server!.url, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        ...(root ? { 'X-Prime-Workspace': Buffer.from(root).toString('base64url') } : {})
      },
      body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params: { name: 'workspace_probe' } })
    })
    const body = await response.text()
    return { status: response.status, body: body ? JSON.parse(body) : null }
  }
  expect((await call(undefined, 'wrong')).status).toBe(401)
  expect(
    (await call()).body.result.tools.some(
      (tool: { name: string }) => tool.name === 'workspace_probe'
    )
  ).toBe(false)
  expect(
    (await call('C:/other')).body.result.tools.some(
      (tool: { name: string }) => tool.name === 'workspace_probe'
    )
  ).toBe(false)
  expect((await call('C:/project', server.token, 'tools/call')).body.result.content).toEqual([
    { type: 'text', text: 'project-a' }
  ])
})

it('binds terminal tools on the server and never trusts a supplied workspace or input owner', async () => {
  const manager = new TerminalManager({ emit: () => {} })
  managers.push(manager)
  const first = manager.create('project-a', process.cwd())
  const other = manager.create('project-b', process.cwd())
  const tools = workbenchTerminalTools(manager)
  const list = tools.find((tool) => tool.name === 'terminal_list')!
  expect(list.inputSchema.required).toEqual([])
  const result = await list.run({ workspaceId: 'project-b' }, { workspaceId: 'project-a' })
  expect(
    JSON.parse((result[0] as { text: string }).text).map((item: { id: string }) => item.id)
  ).toEqual([first.id])
  const read = tools.find((tool) => tool.name === 'terminal_read')!
  await expect(
    read.run({ id: other.id, workspaceId: 'project-b' }, { workspaceId: 'project-a' })
  ).rejects.toThrow(/workspace/i)
  manager.setOwner(first.id, 'agent')
  expect(() =>
    terminalOperation(manager, 'project-a', process.cwd(), {
      action: 'write',
      id: first.id,
      data: '\r',
      source: 'agent'
    })
  ).toThrow(/ownership/i)
  expect(() =>
    terminalOperation(manager, 'project-b', process.cwd(), { action: 'read', id: first.id })
  ).toThrow(/workspace/i)
})
