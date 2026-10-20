import { useEffect, useRef } from 'react'
import type { ConversationState } from './useConversation'
import { drawOrb } from './orbRenderer'
import { readOrbAudio } from './orbAudio'
import { createLiquidRenderer, isHolidayVoice } from './liquidRenderer'
import { drawLiquidFallback } from './liquidFallback'

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
    const liquid = createLiquidRenderer()
    const source = state === 'speaking' ? outputAnalyser : muted ? null : analyser
    const frequencies = new Uint8Array(source?.frequencyBinCount ?? 128)
    const samples = new Uint8Array(source?.fftSize ?? 256)
    let bands = new Array<number>(bandCount).fill(0)
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
      const token = (name: string, fallback: string) =>
        styles.getPropertyValue(name).trim() || fallback
      colors = [
        token('--voice-a', '#ff8c60'),
        token('--voice-b', '#ff4f6d'),
        token('--voice-c', '#ffb35c')
      ]
      dark = document.documentElement.dataset.theme !== 'light'
    }
    const draw = (now: number) => {
      const delta = Math.min(0.05, (now - last) / 1000)
      last = now
      time += reduced ? 0 : delta
      if (source) {
        source.getByteFrequencyData(frequencies)
        source.getByteTimeDomainData(samples)
        const audio = readOrbAudio(frequencies, samples, level, bands)
        level = audio.level
        bands = audio.bands
      } else {
        level *= 0.88
        bands = bands.map((value) => value * 0.9)
      }
      const orbFrame = {
        state,
        muted,
        level: reduced ? 0 : level,
        bands: reduced ? [] : bands,
        time
      }
      if (!isHolidayVoice()) {
        if (liquid) liquid.draw(context, element.width, orbFrame, dark)
        else drawLiquidFallback(context, element.width, orbFrame, dark)
      } else drawOrb(context, element.width, orbFrame, colors, dark)
      if (!reduced) frame = requestAnimationFrame(draw)
    }
    const update = () => {
      cancelAnimationFrame(frame)
      reduced = preference.matches || document.documentElement.dataset.motion === 'reduced'
      resize()
      readColors()
      last = performance.now()
      if (!document.hidden) frame = requestAnimationFrame(draw)
    }
    const observer = new MutationObserver(update)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-motion', 'data-theme']
    })
    const sizeObserver = new ResizeObserver(update)
    sizeObserver.observe(element)
    preference.addEventListener('change', update)
    document.addEventListener('visibilitychange', update)
    update()
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      liquid?.dispose()
      sizeObserver.disconnect()
      preference.removeEventListener('change', update)
      document.removeEventListener('visibilitychange', update)
    }
  }, [analyser, outputAnalyser, muted, state])
  return (
    <div
      className={`voice-orb is-${state}${muted && state !== 'speaking' ? ' is-muted' : ''}`}
      aria-hidden="true"
    >
      <canvas ref={canvas} />
    </div>
  )
}
