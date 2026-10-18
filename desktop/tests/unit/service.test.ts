import { expect, it } from 'vitest'
import { PrimeService } from '../../src/main/prime/service'
import { PrimeTransport } from '../../src/main/prime/transport'

function service(transport: PrimeTransport) {
  const instance = new PrimeService(transport, () => {})
  instance.projects.set('workspace', {
    id: 'workspace',
    path: 'C:/fixture',
    clientId: 'client',
    connection: 'ready'
  })
  return instance
}
it('keeps an ambiguously accepted run active when both network requests fail', async () => {
  class OfflineTransport extends PrimeTransport {
    override async request<T>(): Promise<T> {
      throw new Error('connection lost')
    }
  }
  const instance = service(new OfflineTransport('unused'))
  await expect(
    instance.call('workspace', 'send', { sessionId: 'session', runId: 'run', prompt: 'test' })
  ).rejects.toThrow('connection lost')
  expect(instance.isBusy()).toBe(true)
})
it('does not retain activity for a synchronously rejected request', async () => {
  class RejectedTransport extends PrimeTransport {
    override async request<T>(): Promise<T> {
      throw Object.assign(new Error('invalid request'), { status: 400 })
    }
  }
  const instance = service(new RejectedTransport('unused'))
  await expect(
    instance.call('workspace', 'send', { sessionId: 'session', runId: 'run', prompt: 'test' })
  ).rejects.toThrow()
  expect(instance.isBusy()).toBe(false)
})
it('tracks and cancels direct shell HTTP work independently of agent activity', async () => {
  class ShellTransport extends PrimeTransport {
    override async request<T>(
      _method: string,
      path: string,
      _body?: unknown,
      _timeout?: number,
      signal?: AbortSignal
    ): Promise<T> {
      if (path.endsWith('/shell'))
        return new Promise((_resolve, reject) =>
          signal?.addEventListener('abort', () => reject(new Error('cancelled')))
        )
      return null as T
    }
  }
  const instance = service(new ShellTransport('unused'))
  const request = instance.call('workspace', 'shell', {
    sessionId: 'session',
    command: 'long command'
  })
  expect(instance.isBusy()).toBe(true)
  const rejected = expect(request).rejects.toThrow('cancelled')
  await instance.call('workspace', 'cancel', { sessionId: 'session' })
  await rejected
  expect(instance.isBusy()).toBe(false)
})
