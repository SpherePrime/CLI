import { describe, expect, it } from 'vitest'
import { createNeuralScene } from '../../src/renderer/src/features/voice/orbGeometry'
import { readOrbAudio } from '../../src/renderer/src/features/voice/orbAudio'

const silent = {
  state: 'listening' as const,
  muted: false,
  level: 0,
  bands: new Array(12).fill(0),
  time: 2
}

describe('neural voice geometry', () => {
  it('modulates individual filaments from the frequency spectrum at equal volume', () => {
    const bass = createNeuralScene({ ...silent, bands: [1, ...new Array(11).fill(0)] })
    const treble = createNeuralScene({ ...silent, bands: [...new Array(11).fill(0), 1] })
    expect(bass.strands[0].points).not.toEqual(treble.strands[0].points)
  })
  it('expands the neural volume with actual voice energy', () => {
    const quiet = createNeuralScene(silent)
    const loud = createNeuralScene({ ...silent, level: 1 })
    expect(loud.radius).toBeGreaterThan(quiet.radius * 1.08)
    expect(loud.strands.length).toBeLessThanOrEqual(80)
    for (const strand of loud.strands)
      for (const point of strand.points) {
        expect(Number.isFinite(point.x + point.y + point.z)).toBe(true)
        expect(Math.hypot(point.x, point.y)).toBeLessThan(1.5)
      }
  })
  it('keeps speaker visualization active while the microphone is muted', () => {
    const frame = { ...silent, state: 'speaking' as const, level: 0.8 }
    expect(createNeuralScene({ ...frame, muted: true })).toEqual(createNeuralScene(frame))
  })
})

it('derives level and bands from analyser samples and releases them in silence', () => {
  const frequencies = new Uint8Array(128).fill(180)
  const samples = new Uint8Array(256).map(
    (_, index) => 128 + Math.round(Math.sin(index * 0.3) * 42)
  )
  const active = readOrbAudio(frequencies, samples, 0, new Array(12).fill(0))
  expect(active.level).toBeGreaterThan(0.3)
  expect(active.bands.every((value) => value > 0)).toBe(true)
  const quiet = readOrbAudio(
    new Uint8Array(128),
    new Uint8Array(256).fill(128),
    active.level,
    active.bands
  )
  expect(quiet.level).toBeLessThan(active.level)
  expect(quiet.bands[0]).toBeLessThan(active.bands[0])
})
