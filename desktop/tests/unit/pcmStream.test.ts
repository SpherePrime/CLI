import { describe, expect, it } from 'vitest'
import { bytesToBase64, downsampleTo16k, float32ToS16LE } from '../../src/renderer/src/features/voice/pcmStream'
import { normalizeAsrLanguage } from '../../src/shared/stt'

describe('pcmStream helpers', () => {
  it('downsamples 48kHz to 16kHz preserving constant signal', () => {
    const input = new Float32Array(4800).fill(0.25)
    const out = downsampleTo16k(input, 48_000)
    expect(out.length).toBe(1600)
    expect(out[0]).toBeCloseTo(0.25, 4)
    expect(out[1599]).toBeCloseTo(0.25, 4)
  })

  it('converts float32 samples into clamped s16le bytes', () => {
    const bytes = float32ToS16LE(new Float32Array([-2, -1, 0, 0.5, 1, 2]))
    const view = new DataView(bytes.buffer)
    expect(view.getInt16(0, true)).toBe(-32768)
    expect(view.getInt16(2, true)).toBe(-32768)
    expect(view.getInt16(4, true)).toBe(0)
    expect(view.getInt16(6, true)).toBe(16384)
    expect(view.getInt16(8, true)).toBe(32767)
    expect(view.getInt16(10, true)).toBe(32767)
  })

  it('encodes PCM bytes to base64 accurately', () => {
    const encoded = bytesToBase64(new Uint8Array([0x01, 0x02, 0x03, 0xff]))
    expect(Buffer.from(encoded, 'base64')).toEqual(Buffer.from([0x01, 0x02, 0x03, 0xff]))
  })

  it('normalizes ASR language codes to auto/ru/en/uk', () => {
    expect(normalizeAsrLanguage('')).toBe('auto')
    expect(normalizeAsrLanguage('auto')).toBe('auto')
    expect(normalizeAsrLanguage('ru-RU')).toBe('ru')
    expect(normalizeAsrLanguage('en_US')).toBe('en')
    expect(normalizeAsrLanguage('uk-UA')).toBe('uk')
    expect(normalizeAsrLanguage('ua')).toBe('uk')
    expect(normalizeAsrLanguage('fr')).toBe('auto')
  })
})
