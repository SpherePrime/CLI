import { afterEach, expect, it, vi } from 'vitest'
import { Speaker } from '../../src/renderer/src/features/voice/speaker'

afterEach(() => vi.unstubAllGlobals())

it('routes the played audio through the exposed analyser and releases it on disposal', async () => {
  const analyser = { fftSize: 0, smoothingTimeConstant: 0, connect: vi.fn(), disconnect: vi.fn() }
  const destination = {}
  const source = {
    buffer: null,
    onended: undefined as (() => void) | undefined,
    connect: vi.fn(),
    start: () => queueMicrotask(() => source.onended?.()),
    stop: vi.fn()
  }
  const close = vi.fn(async () => {})
  vi.stubGlobal(
    'AudioContext',
    class {
      destination = destination
      state = 'running'
      createAnalyser() {
        return analyser
      }
      createBufferSource() {
        return source
      }
      decodeAudioData() {
        return Promise.resolve({})
      }
      close = close
    }
  )
  vi.stubGlobal('window', { prime: { speech: async () => ({ audio: 'AA==' }) } })
  const states: boolean[] = []
  const speaker = new Speaker({
    voice: 'F1',
    speed: 1,
    onState: (value) => states.push(value),
    onError: (error) => {
      throw error
    }
  })
  await speaker.speak(['Hello'])
  expect(speaker.analyser).toBe(analyser)
  expect(source.connect).toHaveBeenCalledWith(analyser)
  expect(analyser.connect).toHaveBeenCalledWith(destination)
  expect(states).toEqual([true, false])
  await speaker.dispose()
  expect(analyser.disconnect).toHaveBeenCalledOnce()
  expect(close).toHaveBeenCalledOnce()
})

it('never plays synthesized audio that arrives after interruption', async () => {
  let complete!: (value: { audio: string }) => void
  const start = vi.fn()
  vi.stubGlobal(
    'AudioContext',
    class {
      destination = {}
      state = 'running'
      createAnalyser() {
        return { connect() {}, disconnect() {} }
      }
      createBufferSource() {
        return { connect() {}, start }
      }
      decodeAudioData() {
        return Promise.resolve({})
      }
      close() {
        return Promise.resolve()
      }
    }
  )
  vi.stubGlobal('window', {
    prime: {
      speech: () =>
        new Promise((resolve) => {
          complete = resolve
        })
    }
  })
  const states: boolean[] = []
  const speaker = new Speaker({
    voice: 'F1',
    speed: 1,
    onState: (value) => states.push(value),
    onError: () => {}
  })
  const pending = speaker.speak(['Hello'])
  speaker.stop()
  complete({ audio: 'AA==' })
  await pending
  expect(start).not.toHaveBeenCalled()
  expect(states).toEqual([true, false])
  await speaker.dispose()
})
