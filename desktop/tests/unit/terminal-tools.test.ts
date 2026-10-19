import { afterEach, expect, it } from 'vitest'
import { TerminalManager } from '../../src/main/terminal/manager'
import { terminalTools } from '../../src/main/terminal/tools'

const manager = new TerminalManager({ emit: () => {} })
afterEach(() => manager.dispose())

it('offers read and gated write operations without an agent ownership escalation tool', async () => {
  const terminal = manager.create('workspace-a', process.cwd())
  const tools = terminalTools(manager)
  expect(tools.map((tool) => tool.name)).toEqual([
    'terminal_list',
    'terminal_read',
    'terminal_write',
    'terminal_resize'
  ])
  const write = tools.find((tool) => tool.name === 'terminal_write')!
  await expect(
    write.run({ workspaceId: 'workspace-a', id: terminal.id, data: '\r' })
  ).rejects.toThrow(/ownership/i)
  await expect(
    write.run({ workspaceId: 'workspace-b', id: terminal.id, data: '\r' })
  ).rejects.toThrow(/workspace/i)
  await expect(write.run({ workspaceId: 'workspace-a', id: terminal.id, data: 0 })).rejects.toThrow(
    /data/i
  )
  manager.setOwner(terminal.id, 'agent')
  await expect(
    write.run({ workspaceId: 'workspace-a', id: terminal.id, data: '\r' })
  ).resolves.toEqual(expect.any(Array))
  const read = tools.find((tool) => tool.name === 'terminal_read')!
  await expect(
    read.run({ workspaceId: 'workspace-a', id: terminal.id }, 'workspace-b')
  ).rejects.toThrow(/workspace/i)
})
