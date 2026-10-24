import { describe, expect, it } from 'vitest'
import {
  dictationLevel,
  smoothDictationLevel
} from '../../src/renderer/src/features/chat/dictationSignal'

describe('dictation signal', () => {
  it('keeps microphone silence still and bounds loud input', () => {
    expect(dictationLevel(new Uint8Array(256).fill(128))).toBe(0)
    expect(dictationLevel(new Uint8Array())).toBe(0)
    expect(dictationLevel(new Uint8Array(256).fill(255))).toBe(1)
    expect(dictationLevel(new Uint8Array(256).fill(140))).toBeGreaterThan(0.1)
  })

  it('responds quickly to speech and releases gently without depending on frame rate', () => {
    const attack = smoothDictationLevel(0, 1, 1 / 60)
    const release = 1 - smoothDictationLevel(1, 0, 1 / 60)
    expect(attack).toBeGreaterThan(release)
    expect(attack).toBeLessThan(1)
    const halfStep = smoothDictationLevel(0, 1, 1 / 120)
    expect(smoothDictationLevel(halfStep, 1, 1 / 120)).toBeCloseTo(attack, 8)
    expect(smoothDictationLevel(0.5, 1, 0)).toBe(0.5)
  })
})
