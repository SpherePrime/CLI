import { useEffect, useRef, useState } from 'react'
import { LoaderCircle, Mic, X } from 'lucide-react'
import { useI18n } from '../../i18n'

const BAR_COUNT = 52
const BAR_STEP = 5

export function VoiceWave({
  analyser,
  transcribing,
  compact = false,
  onCancel
}: {
  analyser: AnalyserNode | null
  transcribing: boolean
  compact?: boolean
  onCancel(): void
}) {
  const { t } = useI18n()
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const startedAt = useRef(Date.now())
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    startedAt.current = Date.now()
    setElapsed(0)
    if (transcribing) return
    const timer = window.setInterval(() => {
      setElapsed(Math.floor((Date.now() - startedAt.current) / 1000))
    }, 500)
    return () => window.clearInterval(timer)
  }, [transcribing])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || transcribing) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const history = new Float32Array(BAR_COUNT)
    const samples = analyser ? new Uint8Array(analyser.fftSize) : null
    let frame = 0
    let lastSample = 0
    let accent = ''
    let muted = ''
    const refreshColors = () => {
      const style = getComputedStyle(canvas)
      accent = style.getPropertyValue('--accent').trim() || '#e86c3a'
      muted = style.getPropertyValue('--secondary').trim() || '#8b8b93'
    }
    refreshColors()

    const draw = (now: number) => {
      const rect = canvas.getBoundingClientRect()
      const dpr = window.devicePixelRatio || 1
      const width = Math.max(1, Math.round(rect.width * dpr))
      const height = Math.max(1, Math.round(rect.height * dpr))
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width
        canvas.height = height
      }
      if (now - lastSample >= 42) {
        lastSample = now
        let level = 0
        if (analyser && samples) {
          analyser.getByteTimeDomainData(samples)
          let sum = 0
          for (let i = 0; i < samples.length; i++) {
            const v = (samples[i] - 128) / 128
            sum += v * v
          }
          level = Math.min(1, Math.sqrt(sum / samples.length) * 5.2)
        }
        history.copyWithin(0, 1)
        history[BAR_COUNT - 1] = level
      }

      ctx.clearRect(0, 0, width, height)
      const mid = height / 2
      const barW = Math.max(2, Math.round(2 * dpr))
      const step = Math.max(barW + dpr, Math.floor(width / BAR_COUNT))
      const count = Math.min(BAR_COUNT, Math.floor(width / step))
      const startX = Math.floor((width - count * step) / 2)
      const maxH = height * 0.82
      const minH = Math.max(2 * dpr, 2.5)

      for (let i = 0; i < count; i++) {
        const level = history[BAR_COUNT - count + i]
        const edge = Math.min(1, Math.min(i, count - 1 - i) / 5)
        const h = Math.max(minH, level * maxH * (0.4 + 0.6 * edge))
        const x = startX + i * step
        const y = mid - h / 2
        ctx.fillStyle = level > 0.06 ? accent : muted
        ctx.globalAlpha = level > 0.06 ? 0.55 + Math.min(0.45, level * 0.65) : 0.28 * edge
        ctx.beginPath()
        ctx.roundRect(x, y, barW, h, barW / 2)
        ctx.fill()
      }
      ctx.globalAlpha = 1
      frame = window.requestAnimationFrame(draw)
    }

    frame = window.requestAnimationFrame(draw)
    return () => window.cancelAnimationFrame(frame)
  }, [analyser, transcribing])

  const minutes = Math.floor(elapsed / 60)
  const seconds = String(elapsed % 60).padStart(2, '0')

  return (
    <div
      className={`voice-wave${transcribing ? ' busy' : ''}${compact ? ' compact' : ''}`}
      role="status"
      aria-live="polite"
    >
      <button
        type="button"
        className="icon"
        title={t('Отменить запись (Esc)')}
        aria-label={t('Отменить запись')}
        onClick={onCancel}
      >
        <X size={15} />
      </button>
      <span className="voice-wave-badge" aria-hidden="true">
        {transcribing ? (
          <LoaderCircle size={15} className="spin" />
        ) : (
          <Mic size={15} />
        )}
      </span>
      <div className="voice-wave-body">
        {transcribing ? (
          <span className="voice-wave-label">{t('Распознавание…')}</span>
        ) : (
          <>
            <span className="voice-wave-dot" aria-hidden="true" />
            <span className="voice-wave-label">{t('Говорите…')}</span>
            <canvas
              ref={canvasRef}
              className="voice-wave-canvas"
              style={{ minWidth: BAR_COUNT * BAR_STEP * 0.4 }}
              aria-hidden="true"
            />
          </>
        )}
      </div>
      {!transcribing && (
        <span className="voice-wave-time" aria-label={t('Длительность записи')}>
          {minutes}:{seconds}
        </span>
      )}
    </div>
  )
}
