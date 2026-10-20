import type { OrbFrame } from './orbRenderer'

export function drawLiquidFallback(
  context: CanvasRenderingContext2D,
  size: number,
  frame: OrbFrame,
  dark: boolean
) {
  context.clearRect(0, 0, size, size)
  context.save()
  context.translate(size / 2, size / 2)
  const radius = size * (0.38 + frame.level * 0.012)
  context.beginPath()
  context.arc(0, 0, radius, 0, Math.PI * 2)
  const glass = context.createRadialGradient(-radius * 0.3, -radius * 0.5, 0, 0, 0, radius)
  glass.addColorStop(0, dark ? '#4e5358' : '#e2ddd3')
  glass.addColorStop(0.5, '#14181d')
  glass.addColorStop(0.94, '#292727')
  glass.addColorStop(1, '#b1a091')
  context.fillStyle = glass
  context.fill()
  context.clip()
  for (let index = 0; index < 4; index++) {
    context.save()
    context.rotate(frame.time * 0.08 + index * 1.5)
    const gradient = context.createLinearGradient(-radius, 0, radius, 0)
    gradient.addColorStop(0, '#edbc7860')
    gradient.addColorStop(0.5, '#fff0ce')
    gradient.addColorStop(1, '#8b481530')
    context.strokeStyle = gradient
    context.lineWidth = radius * (0.04 + frame.level * 0.02)
    context.beginPath()
    context.ellipse(0, 0, radius * 0.96, radius * 0.33, 0.45, 0, Math.PI * 2)
    context.stroke()
    context.restore()
  }
  context.restore()
}
