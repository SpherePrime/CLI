import { afterEach, expect, it, vi } from 'vitest'
import { ShutdownController } from '../../src/main/shutdown'

afterEach(() => vi.useRealTimers())

it('continues shutdown after a failed or stalled service and shares concurrent cleanup', async () => {
  vi.useFakeTimers()
  const closed: string[] = []
  const failures: string[] = []
  const shutdown = new ShutdownController(
    [
      { name: 'automations', close: () => new Promise<void>(() => {}) },
      {
        name: 'remote',
        close: () => {
          throw new Error('Disconnected')
        }
      },
      {
        name: 'runtime',
        close: () => {
          closed.push('runtime')
        }
      }
    ],
    (name) => failures.push(name),
    100
  )
  const first = shutdown.run()
  const second = shutdown.run()
  await vi.advanceTimersByTimeAsync(100)
  await Promise.all([first, second])
  await shutdown.run()
  expect(closed).toEqual(['runtime'])
  expect(failures).toEqual(['automations', 'remote'])
  expect(vi.getTimerCount()).toBe(0)
})
