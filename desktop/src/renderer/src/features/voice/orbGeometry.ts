import type { OrbFrame } from './orbRenderer'

export type NeuralPoint = { x: number; y: number; z: number }
export type NeuralStrand = { points: NeuralPoint[]; color: number; impulse: number }

const strandCount = 64
const segmentCount = 72
const tau = Math.PI * 2

export function createNeuralScene(frame: OrbFrame) {
  const muted = frame.muted && frame.state !== 'speaking'
  const level = muted ? 0 : frame.level
  const processing = ['preparing', 'waiting', 'transcribing'].includes(frame.state)
  const time = frame.time * (processing ? 0.3 : 0.12)
  const radius = 0.98 + level * 0.14
  const turn = time + 0.45
  const tilt = -0.4 + Math.sin(time * 0.6) * 0.12
  const strands: NeuralStrand[] = []
  for (let strand = 0; strand < strandCount; strand++) {
    const latitude = -1.43 + (strand / (strandCount - 1)) * 2.86
    const phase = strand * 2.399963
    const band = muted ? 0 : (frame.bands[strand % Math.max(1, frame.bands.length)] ?? 0)
    const impulse = Math.min(1, level * 0.5 + band * 0.85)
    const points: NeuralPoint[] = []
    for (let segment = 0; segment <= segmentCount; segment++) {
      const angle = (segment / segmentCount) * tau
      const wave = Math.sin(angle * 3 + phase + time * 2) * (0.055 + band * 0.13)
      const polar = latitude + wave * Math.cos(latitude)
      const twist = angle + Math.sin(polar * 3 + time) * 0.2 + turn
      const envelope = 1 + Math.sin(angle * 5 + phase - time * 3) * (0.012 + impulse * 0.035)
      const x = Math.cos(polar) * Math.cos(twist) * envelope
      const y = Math.sin(polar) * envelope
      const z = Math.cos(polar) * Math.sin(twist) * envelope
      const tiltedY = y * Math.cos(tilt) - z * Math.sin(tilt)
      const tiltedZ = y * Math.sin(tilt) + z * Math.cos(tilt)
      const perspective = 3.8 / (3.8 - tiltedZ * 0.36)
      points.push({ x: x * perspective * radius, y: tiltedY * perspective * radius, z: tiltedZ })
    }
    strands.push({ points, color: strand % 9 < 2 ? 3 : strand % 3, impulse })
  }
  return { strands, radius, fade: muted ? 0.36 : 1, processing }
}
