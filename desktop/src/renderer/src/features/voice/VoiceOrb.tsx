import { useEffect, useRef } from 'react'
import type { ConversationState } from './useConversation'
import { drawOrb } from './orbRenderer'
import { readOrbAudio } from './orbAudio'
import { createParticleRenderer } from './particleRenderer'
import { blendLook, coreLook } from './coreStates'

const bandCount = 12
const average = (values: number[], from: number, to: number) =>
  values.slice(from, to).reduce((sum, value) => sum + value, 0) / Math.max(1, to - from)

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
  const live = useRef({ state, muted, analyser, outputAnalyser })
  live.current = { state, muted, analyser, outputAnalyser }
  useEffect(() => {
    const element = canvas.current
    if (!element) return
    const core = createParticleRenderer(element)
    const fallback = core ? null : element.getContext('2d')
    const preference = matchMedia('(prefers-reduced-motion: reduce)')
    let look = coreLook(live.current.state)
    let bands = new Array<number>(bandCount).fill(0)
    let level = 0
    let dim = 0
    let time = 0
    let frame = 0
    let last = performance.now()
    let reduced = false
    let buffers = { frequencies: new Uint8Array(128), samples: new Uint8Array(256) }
    const resize = () => {
      const size = Math.round(Math.min(1100, element.clientWidth * Math.min(2, devicePixelRatio || 1)))
      if (size && element.width !== size) {
        element.width = size
        element.height = size
      }
    }
    const listen = () => {
      const { state, muted, analyser, outputAnalyser } = live.current
      const source = state === 'speaking' ? outputAnalyser : muted ? null : analyser
      if (!source) {
        level *= 0.9
        bands = bands.map((value) => value * 0.9)
        return
      }
      if (buffers.frequencies.length !== source.frequencyBinCount || buffers.samples.length !== source.fftSize)
        buffers = { frequencies: new Uint8Array(source.frequencyBinCount), samples: new Uint8Array(source.fftSize) }
      source.getByteFrequencyData(buffers.frequencies)
      source.getByteTimeDomainData(buffers.samples)
      const audio = readOrbAudio(buffers.frequencies, buffers.samples, level, bands)
      level = audio.level
      bands = audio.bands
    }
    const draw = (now: number) => {
      const delta = Math.min(0.05, (now - last) / 1000)
      last = now
      const { state, muted } = live.current
      const target = coreLook(state)
      look = blendLook(look, target, reduced ? 1 : Math.min(1, delta * 2.6))
      dim += ((muted && state !== 'speaking' ? 1 : 0) - dim) * Math.min(1, delta * 3)
      time += reduced ? 0 : delta * look.speed
      listen()
      const energy = reduced ? 0 : level
      if (core)
        core.draw({
          time,
          energy,
          bass: reduced ? 0 : average(bands, 0, 3),
          treble: reduced ? 0 : average(bands, 7, bandCount),
          dim,
          light: document.documentElement.dataset.theme === 'light' ? 1 : 0,
          bands: reduced ? [] : bands,
          look
        })
      else if (fallback)
        drawOrb(
          fallback,
          element.width,
          { state, muted, level: energy, bands: reduced ? [] : bands, time },
          ['#ff8c60', '#ff4f6d', '#ffb35c'],
          document.documentElement.dataset.theme !== 'light'
        )
      if (!reduced) frame = requestAnimationFrame(draw)
    }
    const start = () => {
      cancelAnimationFrame(frame)
      reduced = preference.matches || document.documentElement.dataset.motion === 'reduced'
      resize()
      last = performance.now()
      if (!document.hidden) frame = requestAnimationFrame(draw)
    }
    const observer = new MutationObserver(start)
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-motion', 'data-theme'] })
    const sizeObserver = new ResizeObserver(start)
    sizeObserver.observe(element)
    preference.addEventListener('change', start)
    document.addEventListener('visibilitychange', start)
    start()
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      sizeObserver.disconnect()
      preference.removeEventListener('change', start)
      document.removeEventListener('visibilitychange', start)
      core?.dispose()
    }
  }, [])
  return (
    <div className={`voice-orb is-${state}${muted && state !== 'speaking' ? ' is-muted' : ''}`} aria-hidden="true">
      <canvas ref={canvas} />
    </div>
  )
}
