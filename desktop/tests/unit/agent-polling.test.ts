import { describe, expect, it, vi } from 'vitest'
import { createAgentPolling } from '../../src/renderer/src/features/agents/agentPolling'

describe('agent polling', () => {
  it('does not overlap requests and discards results after disposal', async () => {
    let resolve!: (value: number) => void
    const receive = vi.fn()
    const load = vi.fn(
      () =>
        new Promise<number>((done) => {
          resolve = done
        })
    )
    const polling = createAgentPolling({
      load,
      receive,
      failed: vi.fn(),
      interval: () => 1000,
      hidden: () => false
    })
    polling.refresh()
    polling.refresh()
    expect(load).toHaveBeenCalledTimes(1)
    polling.dispose()
    resolve(42)
    await Promise.resolve()
    expect(receive).not.toHaveBeenCalled()
  })

  it('pauses while hidden and recovers after errors', async () => {
    vi.useFakeTimers()
    let hidden = true
    const load = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValue(2)
    const failed = vi.fn()
    const receive = vi.fn()
    const polling = createAgentPolling({
      load,
      receive,
      failed,
      interval: () => 1000,
      hidden: () => hidden
    })
    polling.refresh()
    expect(load).not.toHaveBeenCalled()
    hidden = false
    polling.refresh()
    await vi.advanceTimersByTimeAsync(0)
    expect(failed).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1000)
    expect(receive).toHaveBeenCalledWith(2)
    polling.dispose()
    await vi.advanceTimersByTimeAsync(5000)
    expect(load).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  })
})
