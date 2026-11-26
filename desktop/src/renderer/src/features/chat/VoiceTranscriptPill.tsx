import { useEffect, useRef, useState } from 'react'
import type { LiveTranscript } from '../../../../shared/stt'
import { useI18n } from '../../i18n'

type Particle = {
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  phase: number
}

type Props = {
  transcript: LiveTranscript
  analyser: AnalyserNode | null
  transcribing: boolean
}

function splitWords(value: string): string[] {
  return value.trim().split(/\s+/).filter(Boolean)
}

function readMicLevel(analyser: AnalyserNode | null, buffer: Uint8Array<ArrayBuffer>): number {
  if (!analyser) return 0
  analyser.getByteTimeDomainData(buffer)
  let sum = 0
  for (let i = 0; i < buffer.length; i++) {
    const sample = (buffer[i] - 128) / 128
    sum += sample * sample
  }
  return Math.min(1, Math.sqrt(sum / buffer.length) * 4.5)
}

export function VoiceTranscriptPill({ transcript, analyser, transcribing }: Props) {
  const { t } = useI18n()
  const containerRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [fadedTop, setFadedTop] = useState(false)

  const committedWords = splitWords(transcript.committed)
  const openWords = splitWords(transcript.open)
  const tailWords = splitWords(transcript.tail)
  const hasText = committedWords.length > 0 || openWords.length > 0 || tailWords.length > 0

  useEffect(() => {
    const node = scrollRef.current
    if (!node) return
    node.scrollTop = node.scrollHeight
    setFadedTop(node.scrollHeight > node.clientHeight + 2)
  }, [transcript.committed, transcript.open, transcript.tail])

  useEffect(() => {
    if (!hasText) return
    const container = containerRef.current
    const canvas = canvasRef.current
    if (!container || !canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const reducedMotion =
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches

    const particles: Particle[] = Array.from({ length: 34 }, (_, i) => ({
      x: 0.06 + Math.random() * 0.88,
      y: 0.14 + Math.random() * 0.78,
      vx: (Math.random() - 0.5) * 0.00042,
      vy: (Math.random() - 0.5) * 0.00032,
      radius: 0.75 + Math.random() * 1.25,
      phase: i * 0.73
    }))

    let width = 0
    let height = 0

    const resize = () => {
      const rect = container.getBoundingClientRect()
      width = Math.max(1, rect.width)
      height = Math.max(1, rect.height)
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      canvas.width = Math.max(1, Math.round(width * dpr))
      canvas.height = Math.max(1, Math.round(height * dpr))
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }

    resize()
    const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(resize) : null
    observer?.observe(container)

    const sampleBuffer = new Uint8Array(new ArrayBuffer(analyser?.fftSize || 256))
    let frame = 0
    let phase = 0
    let smoothLevel = 0.16

    const renderFrame = () => {
      if (width <= 1 || height <= 1) {
        if (!reducedMotion) frame = requestAnimationFrame(renderFrame)
        return
      }

      const rawLevel = transcribing ? 0.12 : readMicLevel(analyser, sampleBuffer)
      const targetLevel = Math.max(0.14, rawLevel)
      smoothLevel += (targetLevel - smoothLevel) * 0.12
      phase += reducedMotion ? 0 : 0.028 + smoothLevel * 0.042

      ctx.clearRect(0, 0, width, height)

      const cx1 = width * (0.3 + Math.sin(phase * 0.34) * 0.15)
      const mist1 = ctx.createRadialGradient(cx1, height * 0.92, 4, cx1, height * 0.92, width * 0.44)
      mist1.addColorStop(0, `rgba(240, 122, 73, ${(0.1 + smoothLevel * 0.16).toFixed(3)})`)
      mist1.addColorStop(1, 'rgba(240, 122, 73, 0)')
      ctx.fillStyle = mist1
      ctx.fillRect(0, 0, width, height)

      const cx2 = width * (0.72 + Math.cos(phase * 0.27) * 0.13)
      const mist2 = ctx.createRadialGradient(cx2, height * 0.88, 4, cx2, height * 0.88, width * 0.36)
      mist2.addColorStop(0, `rgba(255, 176, 136, ${(0.06 + smoothLevel * 0.12).toFixed(3)})`)
      mist2.addColorStop(1, 'rgba(255, 176, 136, 0)')
      ctx.fillStyle = mist2
      ctx.fillRect(0, 0, width, height)

      if (!reducedMotion) {
        for (const p of particles) {
          p.x += p.vx * (1 + smoothLevel * 2.2)
          p.y += p.vy * (1 + smoothLevel * 1.6)
          if (p.x < 0.04) p.x = 0.96
          if (p.x > 0.96) p.x = 0.04
          if (p.y < 0.1) p.y = 0.92
          if (p.y > 0.92) p.y = 0.1

          const edgeX = Math.sin(((p.x - 0.04) / 0.92) * Math.PI)
          const edgeY = Math.sin(((p.y - 0.1) / 0.82) * Math.PI)
          const edgeFade = Math.max(0, edgeX * edgeY)
          const pulse = 0.25 + 0.75 * Math.abs(Math.sin(phase * 0.65 + p.phase))
          const alpha = edgeFade * pulse * (0.18 + smoothLevel * 0.34)
          if (alpha <= 0.01) continue

          const px = p.x * width
          const py = p.y * height + Math.sin(phase + p.phase) * smoothLevel * 3.2
          ctx.beginPath()
          ctx.arc(px, py, p.radius, 0, Math.PI * 2)
          ctx.fillStyle = `rgba(255, 196, 162, ${alpha.toFixed(3)})`
          ctx.fill()
        }
        frame = requestAnimationFrame(renderFrame)
      }
    }

    renderFrame()
    return () => {
      cancelAnimationFrame(frame)
      observer?.disconnect()
    }
  }, [hasText, analyser, transcribing])

  if (!hasText) return null

  const tailOffset = committedWords.length + openWords.length

  return (
    <div
      className="voice-transcript-pill"
      ref={containerRef}
      role="status"
      aria-live="polite"
      aria-label={t('Распознаваемый текст')}
    >
      <canvas className="voice-transcript-canvas" ref={canvasRef} aria-hidden="true" />
      <div
        className={`voice-transcript-scroll${fadedTop ? ' faded' : ''}`}
        ref={scrollRef}
      >
        {committedWords.length > 0 && (
          <span className="voice-live-committed">{committedWords.join(' ')}</span>
        )}
        {openWords.length > 0 && (
          <>
            {committedWords.length > 0 && ' '}
            <span className="voice-live-pending">{openWords.join(' ')}</span>
          </>
        )}
        {tailWords.length > 0 && (
          <>
            {(committedWords.length > 0 || openWords.length > 0) && ' '}
            {tailWords.map((word, index) => (
              <span key={`${tailOffset + index}-${word}`}>
                {index > 0 ? ' ' : ''}
                <span
                  className={
                    index === tailWords.length - 1
                      ? 'voice-live-tail voice-live-fresh'
                      : 'voice-live-tail'
                  }
                >
                  {word}
                </span>
              </span>
            ))}
          </>
        )}
        {!transcribing && <span className="voice-live-caret" aria-hidden="true" />}
      </div>
    </div>
  )
}
