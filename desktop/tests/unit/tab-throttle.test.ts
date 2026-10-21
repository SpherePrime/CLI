import { afterEach, expect, it, vi } from 'vitest'
import { TabThrottle } from '../../src/main/browser/tabThrottle'

afterEach(() => vi.useRealTimers())

const contents = () => {
  const calls: boolean[] = []
  return {
    calls,
    isDestroyed: () => false,
    setBackgroundThrottling: (value: boolean) => calls.push(value)
  } as any
}

it('throttles hidden tabs after they stay idle and wakes them on use', () => {
  vi.useFakeTimers()
  const visibleTab = contents()
  const idleTab = contents()
  const throttle = new TabThrottle((item) => item === visibleTab, 1000, 100)
  throttle.touch(visibleTab)
  throttle.touch(idleTab)
  vi.advanceTimersByTime(500)
  expect(idleTab.calls).toEqual([])
  vi.advanceTimersByTime(600)
  expect(idleTab.calls).toEqual([true])
  expect(visibleTab.calls).toEqual([])
  throttle.touch(idleTab)
  expect(idleTab.calls).toEqual([true, false])
  throttle.dispose()
})

it('stops sweeping when no tabs remain', () => {
  vi.useFakeTimers()
  const tab = contents()
  const throttle = new TabThrottle(() => false, 1000, 100)
  throttle.touch(tab)
  throttle.forget(tab)
  expect(vi.getTimerCount()).toBe(0)
})
