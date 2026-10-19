import { useEffect, useRef } from 'react'
import type { ConversationState } from './useConversation'

export function VoiceOrb({
  state,
  muted,
  analyser,
  outputAnalyser
}: {
  state: ConversationState
  muted: boolean
  analyser: AnalyserNode | null
  outputAnalyser: AnalyserNode | null
}) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const orb = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const element = canvas.current
    const shell = orb.current
    if (!element || !shell) return
    const context = element.getContext('2d')
    if (!context) return
    const source = state === 'speaking' ? outputAnalyser : muted ? null : analyser
    const frequencies = new Uint8Array(source?.frequencyBinCount ?? 128)
    const samples = new Uint8Array(source?.fftSize ?? 256)
    const preference = matchMedia('(prefers-reduced-motion: reduce)')
    let frame = 0
    let level = 0
    let reduced = false
    const draw = () => {
      context.clearRect(0, 0, 640, 640)
      let energy = 0
      frequencies.fill(0)
      if (source && !document.hidden) {
        source.getByteFrequencyData(frequencies)
        source.getByteTimeDomainData(samples)
        for (const sample of samples) energy += ((sample - 128) / 128) ** 2
      }
      level += (Math.min(1, Math.sqrt(energy / samples.length) * 4) - level) * 0.22
      shell.style.setProperty('--voice-level', String(reduced ? 0 : level))
      const color = getComputedStyle(element).getPropertyValue('--accent').trim()
      context.strokeStyle = color
      for (let ring = 0; ring < 3; ring++) {
        context.beginPath()
        context.lineWidth = ring === 0 ? 1.8 : 1
        context.globalAlpha = 0.4 - ring * 0.1 + level * 0.22
        for (let step = 0; step <= 256; step++) {
          const angle = (step / 256) * Math.PI * 2
          const bin = Math.floor((Math.min(step, 256 - step) / 128) * frequencies.length * 0.7)
          const frequency = reduced ? 0 : frequencies[bin] / 255
          const radius = 174 + ring * 19 + frequency * (22 + ring * 9) + level * 10
          const x = 320 + Math.cos(angle) * radius
          const y = 320 + Math.sin(angle) * radius
          if (step === 0) context.moveTo(x, y)
          else context.lineTo(x, y)
        }
        context.closePath()
        context.stroke()
      }
      if (!reduced) frame = requestAnimationFrame(draw)
    }
    const update = () => {
      cancelAnimationFrame(frame)
      reduced = preference.matches || document.documentElement.dataset.motion === 'reduced'
      draw()
    }
    const observer = new MutationObserver(update)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-motion', 'data-theme']
    })
    preference.addEventListener('change', update)
    update()
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      preference.removeEventListener('change', update)
    }
  }, [analyser, outputAnalyser, muted, state])
  return (
    <div
      className={`voice-orb is-${state}${muted && state !== 'speaking' ? ' is-muted' : ''}`}
      ref={orb}
      aria-hidden="true"
    >
      <div className="voice-orb-aura" />
      <canvas ref={canvas} width={640} height={640} />
      <div className="voice-orb-core">
        <div className="voice-orb-light" />
      </div>
    </div>
  )
}
