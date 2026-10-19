import { afterEach, expect, it, vi } from 'vitest'
import { Listener } from '../../src/renderer/src/features/voice/listener'

afterEach(() => vi.unstubAllGlobals())

it('releases a microphone permission result that arrives after ending the call', async () => {
  let allow!: (stream: unknown) => void
  const stopTrack = vi.fn()
  const createContext = vi.fn()
  vi.stubGlobal('navigator', {
    mediaDevices: {
      getUserMedia: () =>
        new Promise((resolve) => {
          allow = resolve
        })
    }
  })
  vi.stubGlobal(
    'AudioContext',
    class {
      constructor() {
        createContext()
      }
      destination = {}
      createMediaStreamSource() {
        return { connect() {}, disconnect() {} }
      }
      createAnalyser() {
        return { fftSize: 0, disconnect() {} }
      }
      createScriptProcessor() {
        return { connect() {}, disconnect() {}, onaudioprocess: null }
      }
      close() {
        return Promise.resolve()
      }
    }
  )
  const listener = new Listener({
    silenceMs: 900,
    sensitivity: () => 1,
    onSpeechStart() {},
    onSegment() {}
  })
  const starting = listener.start()
  await listener.stop()
  allow({ getTracks: () => [{ stop: stopTrack }] })
  await starting
  expect(stopTrack).toHaveBeenCalledOnce()
  expect(createContext).not.toHaveBeenCalled()
  expect(listener.analyser).toBeUndefined()
})
