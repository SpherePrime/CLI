import { expect, it, vi } from 'vitest'
import { SshAccess, sshProfileTools } from '../../src/main/remote/access'
import { createSshExecutionTools } from '../../src/main/remote/execution'

function fixture() {
  const connection = {
    root: '/srv/app',
    resolve: vi.fn(async () => '/srv/app'),
    stat: vi.fn(async () => ({ isDirectory: () => true })),
    exec: vi.fn(async () => ({ stdout: 'remote-host', stderr: '', code: 0 }))
  }
  let connected = false
  const manager = {
    list: () => ({
      profiles: [
        {
          id: 'server',
          name: 'Production',
          host: 'example.test',
          port: 22,
          user: 'deploy',
          root: '/srv/app',
          auth: 'password',
          status: connected ? 'connected' : 'disconnected',
          hasSavedCredentials: true,
          password: 'must-not-leak',
          keyPath: '/private/key'
        }
      ]
    }),
    connection: vi.fn(() => {
      if (!connected) throw new Error('remote.notConnected')
      return connection
    }),
    connect: vi.fn(async () => {
      connected = true
    }),
    disconnect: vi.fn(async () => {
      connected = false
    })
  }
  const access = new SshAccess({ manager: () => manager as any, byWorkspace: () => undefined })
  return { access, connection, manager }
}

it('lists saved SSH profiles from an ordinary chat without exposing credentials', async () => {
  const { access, manager } = fixture()
  const tools = sshProfileTools(access)
  const result = await tools
    .find((t) => t.name === 'ssh_profiles')!
    .run({}, { workspaceId: 'local-chat' })
  const text = JSON.stringify(result)
  expect(text).toContain('Production')
  expect(text).not.toContain('must-not-leak')
  expect(text).not.toContain('/private/key')
  expect(manager.connect).not.toHaveBeenCalled()
})

it('executes on an explicitly selected profile without opening a project or another chat', async () => {
  const { access, connection, manager } = fixture()
  const tools = createSshExecutionTools((workspace, profile) => access.resolve(workspace, profile))
  await tools
    .find((t) => t.name === 'ssh_exec')!
    .run({ profileId: 'server', command: 'hostname' }, { workspaceId: 'local-chat' })
  expect(manager.connect).toHaveBeenCalledWith('server', {}, { prepareWorkspace: false })
  expect(connection.exec).toHaveBeenCalledWith(
    expect.stringContaining('hostname'),
    expect.anything()
  )
  expect((tools[0].inputSchema as any).properties.profileId).toBeDefined()
})

it('shares concurrent connection requests and does not select an unrelated server implicitly', async () => {
  const { access, manager } = fixture()
  await Promise.all([access.resolve('chat-one', 'server'), access.resolve('chat-two', 'server')])
  expect(manager.connect).toHaveBeenCalledTimes(1)
  await expect(access.resolve('local-chat')).rejects.toThrow('SSH workspace is disconnected')
  await expect(access.resolve('local-chat', 'unknown')).rejects.toThrow('Unknown SSH profile')
})
