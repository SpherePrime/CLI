import { expect, it, vi } from 'vitest'
import type { PrimeService } from '../../src/main/prime/service'
import type { RemoteProfile } from '../../src/shared/remote'
import { WorkspaceRouter } from '../../src/main/workbench/workspaceRouter'
import { remoteWorkspaceKey } from '../../src/shared/remoteWorkspace'

function backend(id: string) {
  const project = { id, path: '/same/project', clientId: id, connection: 'ready' }
  return { projects: new Map([[id, project]]), call: vi.fn(async () => id), root: () => project.path, isBusy: () => false, open: vi.fn(async () => project), close: vi.fn(), dispose: vi.fn(), detach: vi.fn() } as unknown as PrimeService
}

it('separates remote identities and routes every call to its owning backend', async () => {
  const local = backend('local')
  const remote = backend('remote')
  const profile: RemoteProfile = { id: '11111111-1111-4111-8111-111111111111', name: 'Server', host: 'example.test', port: 22, user: 'user', root: '/same/project', auth: 'agent', runtime: 'bundled' }
  const key = remoteWorkspaceKey(profile.id, profile.root)
  const router = new WorkspaceRouter(local, () => [{ profile, service: remote, workspaceId: 'remote', key }])
  expect(router.projects.get('remote')?.path).toBe(key)
  expect(router.projects.get('local')?.path).toBe('/same/project')
  expect(router.root('remote')).toBe('/same/project')
  expect(await router.call('remote', 'sessions')).toBe('remote')
  expect(await router.call('local', 'sessions')).toBe('local')
  expect((await router.open(key)).id).toBe('remote')
  expect(local.open).not.toHaveBeenCalled()
  await expect(router.call('unknown', 'sessions')).rejects.toThrow()
})

it('keeps disconnected remote roots from ever becoming local workspaces', async () => {
  const local = backend('local')
  const router = new WorkspaceRouter(local, () => [])
  await expect(router.open(remoteWorkspaceKey('11111111-1111-4111-8111-111111111111', '/remote'))).rejects.toThrow('SSH')
  expect(local.open).not.toHaveBeenCalled()
})

it('keeps detached SSH proxies hidden and blocked when another remote shares the local service', async () => {
  const local = backend('local')
  const profile: RemoteProfile = { id: '11111111-1111-4111-8111-111111111111', name: 'Server', host: 'example.test', port: 22, user: 'user', root: '/server/project', auth: 'agent', runtime: 'bundled' }
  const metadata = { profileId: profile.id, name: profile.name, host: profile.host, port: profile.port, user: profile.user, root: profile.root }
  local.projects.set('detached', { id: 'detached', path: 'C:/state/ssh/a', clientId: 'a', connection: 'ready', remote: metadata })
  local.projects.set('connected', { id: 'connected', path: 'C:/state/ssh/b', clientId: 'b', connection: 'ready', remote: metadata })
  const key = remoteWorkspaceKey(profile.id, profile.root)
  const router = new WorkspaceRouter(local, () => [{ profile, service: local, workspaceId: 'connected', key, root: profile.root }])
  expect(router.projects.has('detached')).toBe(false)
  expect(router.projects.get('connected')?.remote?.root).toBe(profile.root)
  expect(router.root('connected')).toBe(profile.root)
  await expect(router.call('detached', 'sessions')).rejects.toThrow('Сервер отключён')
  expect(local.call).not.toHaveBeenCalled()
})
