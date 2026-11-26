import { useEffect, useRef, useState } from 'react'
import { LoaderCircle, Mic, X } from 'lucide-react'
import type { LiveTranscript } from '../../../../shared/stt'
import { useI18n } from '../../i18n'
import '../../styles/dictation.css'

const BAR_WIDTH = 2
const BAR_GAP = 3
const STEP = BAR_WIDTH + BAR_GAP
const SCROLL_INTERVAL = 52
const NOISE_FLOOR = 0.012
const SILENCE_BASELINE = 0.03
const LEVEL_GAIN = 4.6
const MIN_BAR = 2
const BAR_SCALE = 0.44
const MIN_CANVAS_BARS = 8
const INITIAL_BARS = 140
const MAX_DPR = 2

export function VoiceWave({
  analyser,
  transcribing,
  onCancel,
  liveTranscript,
  compact = false
}: {
  analyser: AnalyserNode | null
  transcribing: boolean
  onCancel?: () => void
  liveTranscript?: LiveTranscript
  compact?: boolean
}) {
  const { t } = useI18n()
  const canvas = useRef<HTMLCanvasElement>(null)
  const transcriptBox = useRef<HTMLDivElement>(null)
  const [seconds, setSeconds] = useState(0)

  const committed = liveTranscript?.committed?.trim() || ''
  const open = liveTranscript?.open?.trim() || ''
  const tail = liveTranscript?.tail?.trim() || ''
  const hasLiveText = Boolean(committed || open || tail)

  useEffect(() => {
    if (transcribing) return
    const started = Date.now()
    setSeconds(0)
    const timer = window.setInterval(
      () => setSeconds(Math.floor((Date.now() - started) / 1000)),
      250
    )
    return () => window.clearInterval(timer)
  }, [transcribing])

  useEffect(() => {
    const el = transcriptBox.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [committed, open, tail])

  useEffect(() => {
    const el = canvas.current
    if (!el) return
    const ctx = el.getContext('2d')
    if (!ctx) return
    const samples = analyser ? new Float32Array(analyser.fftSize) : null
    const history: number[] = Array(INITIAL_BARS).fill(SILENCE_BASELINE)
    let raf = 0
    let lastShift = performance.now()
    let peak = 0
    const draw = (now: number) => {
      const rect = el.getBoundingClientRect()
      const dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR)
      const width = Math.max(1, Math.round(rect.width * dpr))
      const height = Math.max(1, Math.round(rect.height * dpr))
      if (el.width !== width || el.height !== height) {
        el.width = width
        el.height = height
      }
      if (analyser && samples && !transcribing) {
        analyser.getFloatTimeDomainData(samples)
        let sum = 0
        for (let i = 0; i < samples.length; i++) sum += samples[i] * samples[i]
        const rms = Math.sqrt(sum / samples.length)
        const level = rms < NOISE_FLOOR ? 0 : Math.min(1, Math.pow(rms * LEVEL_GAIN, 0.7))
        if (level > peak) peak = level
      }
      if (now - lastShift >= SCROLL_INTERVAL) {
        lastShift = now
        history.push(transcribing ? SILENCE_BASELINE : Math.max(SILENCE_BASELINE, peak))
        peak = 0
        const count = Math.max(MIN_CANVAS_BARS, Math.floor(rect.width / STEP))
        while (history.length > count) history.shift()
      }
      ctx.clearRect(0, 0, width, height)
      const color = getComputedStyle(el).color
      const count = Math.max(MIN_CANVAS_BARS, Math.floor(rect.width / STEP))
      const mid = height / 2
      const start = Math.max(0, history.length - count)
      for (let i = 0; i < count; i++) {
        const value = history[start + i] ?? SILENCE_BASELINE
        const amplitude = transcribing
          ? (0.1 + 0.08 * Math.sin(now / 260 + i * 0.25)) * height
          : Math.max(MIN_BAR * dpr, value * height * BAR_SCALE)
        const x = i * STEP * dpr
        const alpha = transcribing ? 0.28 : 0.22 + value * 0.78
        ctx.fillStyle = color
        ctx.globalAlpha = alpha
        ctx.beginPath()
        ctx.roundRect(x, mid - amplitude, BAR_WIDTH * dpr, amplitude * 2, BAR_WIDTH * dpr)
        ctx.fill()
      }
      ctx.globalAlpha = 1
      raf = requestAnimationFrame(draw)
    }
    raf = requestAnimationFrame(draw)
    return () => cancelAnimationFrame(raf)
  }, [analyser, transcribing])

  const time = `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`

  return (
    <div className={`voice-dictation${compact ? ' compact' : ''}`}>
      {hasLiveText && (
        <div
          ref={transcriptBox}
          className="voice-live-transcript"
          role="status"
          aria-live="polite"
          aria-label={t('Распознаваемый текст')}
        >
          {committed && <span className="voice-live-committed">{committed}</span>}
          {committed && (open || tail) && ' '}
          {open && <span className="voice-live-pending">{open}</span>}
          {open && tail && ' '}
          {tail && <span className="voice-live-tail">{tail}</span>}
          {!transcribing && <span className="voice-live-caret" aria-hidden="true" />}
        </div>
      )}
      <div
        className={`voice-wave${transcribing ? ' transcribing' : ''}${compact ? ' compact' : ''}${hasLiveText ? ' has-live-text' : ''}`}
        role="status"
        aria-live="polite"
      >
        {onCancel && (
          <button
            type="button"
            className="icon"
            title={t('Отменить запись')}
            aria-label={t('Отменить запись')}
            onClick={onCancel}
          >
            <X size={15} />
          </button>
        )}
        <span className="voice-wave-symbol" aria-hidden="true">
          {transcribing ? <LoaderCircle size={15} /> : <Mic size={15} />}
        </span>
        <span className="voice-wave-label">
          <i className="voice-wave-dot" aria-hidden="true" />
          {transcribing ? t('Распознавание…') : t('Говорите…')}
        </span>
        <canvas ref={canvas} aria-hidden="true" />
        <span className="voice-wave-time">{time}</span>
      </div>
    </div>
  )
}
