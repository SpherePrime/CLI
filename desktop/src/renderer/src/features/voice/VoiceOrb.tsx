import { useEffect, useRef } from 'react'
import type { ConversationState } from './useConversation'
import { drawOrb } from './orbRenderer'

const bandCount = 12

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
  useEffect(() => {
    const element = canvas.current
    const context = element?.getContext('2d')
    if (!element || !context) return
    const source = state === 'speaking' ? outputAnalyser : muted ? null : analyser
    const frequencies = new Uint8Array(source?.frequencyBinCount ?? 128)
    const samples = new Uint8Array(source?.fftSize ?? 256)
    const bands = new Array<number>(bandCount).fill(0)
    const preference = matchMedia('(prefers-reduced-motion: reduce)')
    let frame = 0
    let level = 0
    let time = 0
    let last = performance.now()
    let reduced = false
    let colors: [string, string, string] = ['#ff8c60', '#ff6a35', '#ffc27a']
    let dark = true
    const resize = () => {
      const size = Math.round(element.clientWidth * Math.min(2, devicePixelRatio || 1))
      if (size && element.width !== size) {
        element.width = size
        element.height = size
      }
    }
    const readColors = () => {
      const styles = getComputedStyle(element)
      const token = (name: string, fallback: string) => styles.getPropertyValue(name).trim() || fallback
      colors = [token('--voice-a', '#ff8c60'), token('--voice-b', '#ff4f6d'), token('--voice-c', '#ffb35c')]
      dark = document.documentElement.dataset.theme !== 'light'
    }
    const draw = (now: number) => {
      const delta = Math.min(0.05, (now - last) / 1000)
      last = now
      time += reduced ? 0 : delta
      let energy = 0
      if (source && !document.hidden) {
        source.getByteFrequencyData(frequencies)
        source.getByteTimeDomainData(samples)
        for (const sample of samples) energy += ((sample - 128) / 128) ** 2
        const span = Math.floor((frequencies.length * 0.6) / bandCount)
        for (let band = 0; band < bandCount; band++) {
          let sum = 0
          for (let index = band * span; index < (band + 1) * span; index++) sum += frequencies[index]
          const value = sum / Math.max(1, span) / 255
          bands[band] += (value - bands[band]) * 0.25
        }
      } else for (let band = 0; band < bandCount; band++) bands[band] *= 0.9
      const target = Math.min(1, Math.sqrt(energy / samples.length) * 4.5)
      level += (target - level) * (target > level ? 0.35 : 0.12)
      drawOrb(context, element.width, { state, muted, level: reduced ? 0 : level, bands: reduced ? [] : bands, time }, colors, dark)
      if (!reduced) frame = requestAnimationFrame(draw)
    }
    const update = () => {
      cancelAnimationFrame(frame)
      reduced = preference.matches || document.documentElement.dataset.motion === 'reduced'
      resize()
      readColors()
      last = performance.now()
      frame = requestAnimationFrame(draw)
    }
    const observer = new MutationObserver(update)
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-motion', 'data-theme'] })
    const sizeObserver = new ResizeObserver(resize)
    sizeObserver.observe(element)
    preference.addEventListener('change', update)
    update()
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      sizeObserver.disconnect()
      preference.removeEventListener('change', update)
    }
  }, [analyser, outputAnalyser, muted, state])
  return (
    <div className={`voice-orb is-${state}${muted && state !== 'speaking' ? ' is-muted' : ''}`} aria-hidden="true">
      <canvas ref={canvas} />
    </div>
  )
}
