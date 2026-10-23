import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { useI18n } from '../../i18n'
import { sampledStyle } from '../voice/frameStyle'

const barWidth = 3
const barGap = 2
const sampleInterval = 60

export function VoiceWave({
  analyser,
  transcribing,
  onCancel,
  compact = false
}: {
  analyser: AnalyserNode | null
  transcribing: boolean
  onCancel: () => void
  compact?: boolean
}) {
  const { t } = useI18n()
  const canvas = useRef<HTMLCanvasElement>(null)
  const [seconds, setSeconds] = useState(0)
  useEffect(() => {
    if (transcribing) return
    const started = Date.now()
    const timer = setInterval(() => setSeconds(Math.floor((Date.now() - started) / 1000)), 250)
    return () => clearInterval(timer)
  }, [transcribing])
  useEffect(() => {
    const element = canvas.current
    if (!element) return
    const context = element.getContext('2d')
    if (!context) return
    const history: number[] = []
    const samples = new Uint8Array(analyser?.fftSize || 256)
    let frame = 0
    let lastSample = 0
    const color = sampledStyle(() => getComputedStyle(element).color)
    const draw = () => {
      const ratio = window.devicePixelRatio || 1
      const width = element.clientWidth
      const height = element.clientHeight
      if (element.width !== width * ratio || element.height !== height * ratio) {
        element.width = width * ratio
        element.height = height * ratio
      }
      context.setTransform(ratio, 0, 0, ratio, 0, 0)
      context.clearRect(0, 0, width, height)
      let level = 0
      if (analyser && !transcribing) {
        analyser.getByteTimeDomainData(samples)
        let sum = 0
        for (const value of samples) sum += ((value - 128) / 128) ** 2
        level = Math.min(1, Math.sqrt(sum / samples.length) * 4)
      }
      const capacity = Math.max(1, Math.floor(width / (barWidth + barGap)))
      const now = performance.now()
      if (now - lastSample >= sampleInterval) {
        history.push(level)
        lastSample = now
      }
      while (history.length > capacity) history.shift()
      context.fillStyle = color()
      const offset = width - history.length * (barWidth + barGap)
      history.forEach((value, index) => {
        const bar = Math.max(2, value * (height - 4))
        const x = offset + index * (barWidth + barGap)
        context.beginPath()
        context.roundRect(x, (height - bar) / 2, barWidth, bar, 1.5)
        context.fill()
      })
      frame = requestAnimationFrame(draw)
    }
    frame = requestAnimationFrame(draw)
    return () => cancelAnimationFrame(frame)
  }, [analyser, transcribing])
  const time = `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`
  return (
    <div
      className={`voice-wave${transcribing ? ' transcribing' : ''}${compact ? ' compact' : ''}`}
      role="status"
    >
      <button
        type="button"
        className="icon small"
        aria-label={t('Отменить запись')}
        title={t('Отменить запись')}
        disabled={transcribing}
        onClick={onCancel}
      >
        <X size={15} />
      </button>
      <span className="voice-wave-label">
        <i className="voice-wave-dot" aria-hidden="true" />
        {transcribing ? t('Распознаём речь…') : t('Говорите…')}
      </span>
      <canvas ref={canvas} aria-hidden="true" />
      <span className="voice-wave-time">{time}</span>
    </div>
  )
}
