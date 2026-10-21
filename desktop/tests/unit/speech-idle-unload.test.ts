import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const engines: { release: ReturnType<typeof vi.fn>; synthesize: ReturnType<typeof vi.fn> }[] = []
let synthesizeGate: Promise<void> = Promise.resolve()

vi.mock('../../src/main/speech/assets', () => ({
  speechModelReady: async () => true,
  speechModelDirectory: () => 'models',
  downloadSpeechModel: async () => 'models'
}))

vi.mock('../../src/main/speech/voices', () => ({
  voiceExists: async () => true,
  listVoices: async () => [],
  importVoice: async () => null,
  removeVoice: async () => {}
}))

vi.mock('../../src/main/speech/supertonic', () => ({
  encodeWav: () => Buffer.from('wav'),
  Supertonic: {
    load: async () => {
      const engine = {
        sampleRate: 44100,
        release: vi.fn(async () => {}),
        synthesize: vi.fn(async () => {
          await synthesizeGate
          return new Float32Array(4)
        }),
        forget: vi.fn()
      }
      engines.push(engine)
      return engine
    }
  }
}))

const { SpeechService } = await import('../../src/main/speech/service')

const idleMs = 10 * 60 * 1000

describe('speech engine idle unload', () => {
  beforeEach(() => {
    engines.length = 0
    synthesizeGate = Promise.resolve()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('releases the engine after ten idle minutes and reloads on demand', async () => {
    const service = new SpeechService(() => {}, async () => undefined)
    await service.handle({ action: 'speak', text: 'привет' } as any)
    expect(service.loaded).toBe(true)
    await vi.advanceTimersByTimeAsync(idleMs - 1000)
    expect(engines[0].release).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1000)
    expect(engines[0].release).toHaveBeenCalledTimes(1)
    expect(service.loaded).toBe(false)
    expect(await service.status()).toEqual({ state: 'ready' })

    await service.handle({ action: 'speak', text: 'снова' } as any)
    expect(engines).toHaveLength(2)
    expect(service.loaded).toBe(true)
  })

  it('restarts the idle timer on every use', async () => {
    const service = new SpeechService(() => {}, async () => undefined)
    await service.handle({ action: 'prepare' } as any)
    await vi.advanceTimersByTimeAsync(idleMs - 60000)
    await service.handle({ action: 'speak', text: 'ещё' } as any)
    await vi.advanceTimersByTimeAsync(idleMs - 60000)
    expect(engines[0].release).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(60000)
    expect(engines[0].release).toHaveBeenCalledTimes(1)
    expect(engines).toHaveLength(1)
  })

  it('keeps the engine while synthesis is still running', async () => {
    const service = new SpeechService(() => {}, async () => undefined, 1000)
    let finish = () => {}
    synthesizeGate = new Promise((resolve) => (finish = resolve))
    const speaking = service.handle({ action: 'speak', text: 'долго' } as any)
    await vi.advanceTimersByTimeAsync(5000)
    expect(service.loaded).toBe(true)
    finish()
    await speaking
    await vi.advanceTimersByTimeAsync(999)
    expect(service.loaded).toBe(true)
    await vi.advanceTimersByTimeAsync(1)
    expect(service.loaded).toBe(false)
    expect(engines[0].release).toHaveBeenCalledTimes(1)
  })

  it('dispose releases immediately and cancels the timer', async () => {
    const service = new SpeechService(() => {}, async () => undefined)
    await service.handle({ action: 'prepare' } as any)
    await service.dispose()
    expect(engines[0].release).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(idleMs)
    expect(engines[0].release).toHaveBeenCalledTimes(1)
  })
})
