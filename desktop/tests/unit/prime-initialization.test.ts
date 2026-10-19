import { expect, it, vi } from 'vitest'
import { PrimeService } from '../../src/main/prime/service'
import type { PrimeTransport } from '../../src/main/prime/transport'

function fixture(ready: boolean) {
  let initialized = ready
  const request = vi.fn(async (method: string, route: string) => {
    if (method === 'GET' && route.endsWith('/agent'))
      return { is_ready: initialized, main_agent: 'plan' }
    if (method === 'POST' && route.endsWith('/agent/init')) {
      initialized = true
      return { is_ready: true }
    }
    return null
  })
  const service = new PrimeService({ request } as unknown as PrimeTransport, () => {})
  service.projects.set('project', {
    id: 'project',
    path: '/project',
    clientId: 'client',
    connection: 'ready'
  })
  return { service, request }
}

it('preserves a ready coordinator and selected agent when background work prepares a session', async () => {
  const { service, request } = fixture(true)
  expect(await service.call('project', 'init')).toEqual({ is_ready: true, main_agent: 'plan' })
  expect(request.mock.calls).toEqual([
    ['GET', '/v1/workspaces/project/agent', undefined, 60000, undefined]
  ])
})

it('shares concurrent initialization and rechecks readiness on later requests', async () => {
  const { service, request } = fixture(false)
  const pending = [service.call('project', 'init'), service.call('project', 'init')]
  expect(service.isBusy()).toBe(true)
  await Promise.all(pending)
  await service.call('project', 'init')
  expect(request.mock.calls.filter(([method]) => method === 'POST')).toHaveLength(1)
  expect(service.isBusy()).toBe(false)
})
