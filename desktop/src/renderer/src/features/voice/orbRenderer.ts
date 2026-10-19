import type { ConversationState } from './useConversation'

export type OrbFrame = {
  state: ConversationState
  muted: boolean
  level: number
  bands: number[]
  time: number
}

type Layer = {
  color: number
  scale: number
  alpha: number
  blur: number
  waves: [number, number, number][]
  drift: number
}

const motion: Record<ConversationState, { speed: number; swirl: number; calm: number }> = {
  off: { speed: 0.4, swirl: 0.05, calm: 0.025 },
  preparing: { speed: 1, swirl: 0.6, calm: 0.05 },
  listening: { speed: 0.7, swirl: 0.12, calm: 0.035 },
  hearing: { speed: 1.25, swirl: 0.22, calm: 0.04 },
  transcribing: { speed: 1.5, swirl: 0.9, calm: 0.06 },
  waiting: { speed: 1.3, swirl: 1.1, calm: 0.065 },
  speaking: { speed: 1.1, swirl: 0.3, calm: 0.045 }
}

const layers: Layer[] = [
  { color: 1, scale: 1.12, alpha: 0.5, blur: 34, drift: 0.9, waves: [[2, 0.5, 0], [3, -0.35, 1.3]] },
  { color: 0, scale: 1, alpha: 0.95, blur: 2, drift: 0.6, waves: [[2, -0.45, 0.7], [3, 0.6, 2.4], [4, -0.25, 4.2]] },
  { color: 1, scale: 0.86, alpha: 0.7, blur: 6, drift: 1.2, waves: [[3, 0.7, 1.1], [2, -0.55, 3.3]] },
  { color: 2, scale: 0.62, alpha: 0.65, blur: 10, drift: 1.5, waves: [[2, 0.8, 0.4], [3, -0.6, 2.9]] }
]

function rgba(color: string, alpha: number) {
  const hex = color.trim().replace('#', '')
  const value = hex.length === 3 ? hex.replace(/./g, (char) => char + char) : hex.slice(0, 6)
  const number = Number.parseInt(value, 16)
  if (!Number.isFinite(number)) return `rgba(255, 140, 96, ${alpha})`
  return `rgba(${(number >> 16) & 255}, ${(number >> 8) & 255}, ${number & 255}, ${alpha})`
}

function trace(
  context: CanvasRenderingContext2D,
  center: number,
  radius: number,
  layer: Layer,
  time: number,
  amplitude: number,
  rotation: number
) {
  const points = 96
  context.beginPath()
  for (let index = 0; index <= points; index++) {
    const angle = (index / points) * Math.PI * 2
    let offset = 0
    for (const [harmonic, velocity, phase] of layer.waves)
      offset += Math.sin(harmonic * angle + time * velocity * 1.7 + phase) / layer.waves.length
    const r = radius * layer.scale * (1 + offset * amplitude)
    const x = center + Math.cos(angle + rotation) * r
    const y = center + Math.sin(angle + rotation) * r
    if (index === 0) context.moveTo(x, y)
    else context.lineTo(x, y)
  }
  context.closePath()
}

export function drawOrb(
  context: CanvasRenderingContext2D,
  size: number,
  frame: OrbFrame,
  colors: [string, string, string],
  dark = true
) {
  const settings = motion[frame.state]
  const time = frame.time * settings.speed
  const bands = frame.bands.length ? frame.bands.reduce((sum, value) => sum + value, 0) / frame.bands.length : 0
  const energy = Math.min(1, frame.level * 0.75 + bands * 0.6)
  const center = size / 2
  const radius = size * 0.28 * (1 + energy * 0.1 + Math.sin(time * 1.4) * 0.012)
  const amplitude = settings.calm + energy * 0.11
  const rotation = frame.time * settings.swirl
  const fade = frame.muted ? 0.4 : 1
  context.clearRect(0, 0, size, size)
  context.save()
  context.globalCompositeOperation = 'source-over'
  for (const layer of layers) {
    const color = colors[layer.color]
    const drift = time * layer.drift
    const focusX = center + Math.cos(drift) * radius * 0.28
    const focusY = center + Math.sin(drift * 0.8) * radius * 0.28 - radius * 0.12
    const gradient = context.createRadialGradient(focusX, focusY, 0, center, center, radius * layer.scale * 1.15)
    gradient.addColorStop(0, rgba(color, 0.95))
    gradient.addColorStop(0.55, rgba(color, 0.7))
    gradient.addColorStop(1, rgba(color, 0))
    context.globalAlpha = layer.alpha * fade * 0.85
    context.filter = `blur(${(layer.blur * size) / 640}px)`
    context.fillStyle = gradient
    trace(context, center, radius, layer, time, amplitude * (layer.blur > 20 ? 1.4 : 1), rotation * layer.drift * 0.4)
    context.fill()
  }
  context.filter = `blur(${(14 * size) / 640}px)`
  const shine = context.createRadialGradient(
    center - radius * 0.3,
    center - radius * 0.38,
    0,
    center - radius * 0.3,
    center - radius * 0.38,
    radius * 0.5
  )
  shine.addColorStop(0, `rgba(255, 255, 255, ${dark ? 0.16 : 0.35})`)
  shine.addColorStop(1, 'rgba(255, 255, 255, 0)')
  context.globalAlpha = fade
  context.globalCompositeOperation = 'source-over'
  context.fillStyle = shine
  context.beginPath()
  context.arc(center - radius * 0.3, center - radius * 0.38, radius * 0.5, 0, Math.PI * 2)
  context.fill()
  context.restore()
  if (frame.state === 'waiting' || frame.state === 'transcribing' || frame.state === 'preparing') {
    context.save()
    context.globalCompositeOperation = dark ? 'screen' : 'source-over'
    for (let index = 0; index < 3; index++) {
      const angle = frame.time * 1.6 + (index * Math.PI * 2) / 3
      const orbit = radius * (1.42 + Math.sin(frame.time * 2 + index) * 0.04)
      const x = center + Math.cos(angle) * orbit
      const y = center + Math.sin(angle) * orbit
      const dot = context.createRadialGradient(x, y, 0, x, y, size * 0.022)
      dot.addColorStop(0, rgba(colors[2], 0.95))
      dot.addColorStop(1, rgba(colors[2], 0))
      context.fillStyle = dot
      context.beginPath()
      context.arc(x, y, size * 0.022, 0, Math.PI * 2)
      context.fill()
    }
    context.restore()
  }
}
