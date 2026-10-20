import type { ConversationState } from './useConversation'
import { createNeuralScene, type NeuralStrand } from './orbGeometry'

export type OrbFrame = {
  state: ConversationState
  muted: boolean
  level: number
  bands: number[]
  time: number
}

function rgba(color: string, alpha: number) {
  const hex = color.trim().replace('#', '')
  const value = hex.length === 3 ? hex.replace(/./g, (char) => char + char) : hex.slice(0, 6)
  const number = Number.parseInt(value, 16)
  return `rgba(${(number >> 16) & 255}, ${(number >> 8) & 255}, ${number & 255}, ${alpha})`
}

function drawHemisphere(
  context: CanvasRenderingContext2D,
  strand: NeuralStrand,
  front: boolean,
  radius: number
) {
  context.beginPath()
  let connected = false
  for (const point of strand.points) {
    const visible = front ? point.z >= 0 : point.z < 0
    if (!visible) {
      connected = false
      continue
    }
    if (connected) context.lineTo(point.x * radius, point.y * radius)
    else context.moveTo(point.x * radius, point.y * radius)
    connected = true
  }
  context.stroke()
}

export function drawOrb(
  context: CanvasRenderingContext2D,
  size: number,
  frame: OrbFrame,
  colors: [string, string, string],
  dark = true
) {
  const scene = createNeuralScene(frame)
  const radius = size * 0.335
  const palette = [...colors, dark ? '#e2f1f3' : '#526d7b']
  context.clearRect(0, 0, size, size)
  context.save()
  context.translate(size / 2, size / 2)
  context.globalAlpha = scene.fade
  const aura = context.createRadialGradient(0, 0, radius * 0.25, 0, 0, radius * 1.38)
  aura.addColorStop(0, rgba(colors[0], dark ? 0.07 : 0.035))
  aura.addColorStop(0.65, rgba(colors[1], dark ? 0.045 : 0.02))
  aura.addColorStop(1, rgba(colors[0], 0))
  context.fillStyle = aura
  context.fillRect(-size / 2, -size / 2, size, size)
  context.lineWidth = size / 900
  for (const strand of scene.strands) {
    context.strokeStyle = rgba(palette[strand.color], dark ? 0.14 : 0.15)
    drawHemisphere(context, strand, false, radius)
  }
  const nucleus = context.createRadialGradient(
    -radius * 0.2,
    -radius * 0.22,
    0,
    0,
    0,
    radius * 0.95
  )
  nucleus.addColorStop(0, rgba(dark ? '#302024' : '#fff8ef', 0.96))
  nucleus.addColorStop(0.55, rgba(dark ? '#21191d' : '#fff8f1', 0.88))
  nucleus.addColorStop(1, rgba(dark ? '#21191d' : '#fff8f1', 0))
  context.fillStyle = nucleus
  context.beginPath()
  context.arc(0, 0, radius, 0, Math.PI * 2)
  context.fill()
  for (const strand of scene.strands) {
    context.lineWidth = (size / 820) * (1 + strand.impulse * 0.4)
    context.strokeStyle = rgba(palette[strand.color], (dark ? 0.47 : 0.46) + strand.impulse * 0.3)
    drawHemisphere(context, strand, true, radius)
    const offset = Math.floor(
      (frame.time * (scene.processing ? 12 : 5) + strand.color * 15) % (strand.points.length - 1)
    )
    const node =
      strand.points[(offset + scene.strands.indexOf(strand) * 13) % (strand.points.length - 1)]
    if (node.z < 0.1) continue
    const x = node.x * radius
    const y = node.y * radius
    const dotRadius = size * (0.002 + strand.impulse * 0.0018)
    const glow = context.createRadialGradient(x, y, 0, x, y, dotRadius * 5)
    glow.addColorStop(0, rgba(palette[strand.color], 0.55 + strand.impulse * 0.4))
    glow.addColorStop(1, rgba(palette[strand.color], 0))
    context.fillStyle = glow
    context.fillRect(x - dotRadius * 5, y - dotRadius * 5, dotRadius * 10, dotRadius * 10)
    context.fillStyle = dark ? '#fff0df' : palette[strand.color]
    context.beginPath()
    context.arc(x, y, dotRadius, 0, Math.PI * 2)
    context.fill()
  }
  context.restore()
}
