import { expect, it, vi } from 'vitest'
import { PrimeService } from '../../src/main/prime/service'
import type { PrimeTransport } from '../../src/main/prime/transport'

it('opens remote POSIX roots through the remote resolver and preserves case sensitivity', async () => {
  let sequence = 0
  const request = vi.fn(async () => ({ id: `remote-${++sequence}` }))
  const unsubscribe = vi.fn()
  const subscribe = vi.fn((_route, _event, state) => { state('ready'); return unsubscribe })
  const resolveRoot = vi.fn(async (path: string) => path)
  const events: unknown[] = []
  const service = new PrimeService({ request, subscribe } as unknown as PrimeTransport, event => events.push(event), { resolveRoot, caseSensitive: true })
  const first = await service.open('/remote/Project')
  const second = await service.open('/remote/project')
  expect(first.id).not.toBe(second.id)
  expect(first.path).toBe('/remote/Project')
  expect(resolveRoot).toHaveBeenCalledWith('/remote/Project')
  service.detach()
  expect(service.projects.size).toBe(0)
  expect(unsubscribe).toHaveBeenCalledTimes(2)
  expect(events).toContainEqual({ workspaceId: first.id, type: 'connection', payload: 'disconnected' })
})
