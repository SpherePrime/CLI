import { expect, it, vi } from 'vitest'
import { RemoteWorkspaces } from '../../src/main/remote/workspaces'

vi.mock('../../src/main/remote/localWorkspace', () => ({
  prepareLocalSshWorkspace: vi.fn(async () => '/local/ssh-proxy')
}))
vi.mock('../../src/main/remote/historyMigration', () => ({
  migrateRemoteHistory: vi.fn(async () => {})
}))

it('does not let a detached connection close a project claimed by its replacement', async () => {
  let ready!: (value: any) => void
  let replacementReady!: (value: any) => void
  const pending = new Promise((resolve) => {
    ready = resolve
  })
  const replacementPending = new Promise((resolve) => {
    replacementReady = resolve
  })
  const service = {
    open: vi.fn().mockReturnValueOnce(pending).mockReturnValueOnce(replacementPending),
    close: vi.fn(async () => {}),
    call: vi.fn(async () => []),
    projects: new Map(),
    isBusy: () => false
  }
  const remotes = new RemoteWorkspaces({
    path: '/unused',
    workspaceDirectory: '/local',
    service: () => service as any,
    emit: () => {},
    channels: () => [],
    onDetached: vi.fn(async () => {})
  })
  const profile = {
    id: 'server',
    name: 'Server',
    host: 'example.test',
    port: 22,
    user: 'deploy',
    root: '/srv/app',
    auth: 'agent',
    runtime: 'bundled'
  }
  const connection = { root: '/srv/app' }
  const previous = (remotes as any).attach(profile, connection)
  await vi.waitFor(() => expect(service.open).toHaveBeenCalledTimes(1))
  const detached = (remotes as any).detach(profile.id)
  const replacement = (remotes as any).attach(profile, connection)
  await new Promise((resolve) => setImmediate(resolve))
  expect(service.open).toHaveBeenCalledTimes(1)
  ready({ id: 'old-project', path: '/local/ssh-proxy' })
  await Promise.all([previous, detached])
  await vi.waitFor(() => expect(service.open).toHaveBeenCalledTimes(2))
  replacementReady({ id: 'new-project', path: '/local/ssh-proxy' })
  await replacement
  expect(service.close.mock.calls).toEqual([['old-project']])
  expect(remotes.byProfile(profile.id)?.workspaceId).toBe('new-project')
})
