import { useEffect, useRef, useState } from 'react'
import { LoaderCircle, Mic, X } from 'lucide-react'
import { useI18n } from '../../i18n'
import { createDictationRenderer } from './dictationRenderer'

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
    return createDictationRenderer(element, analyser, transcribing)
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
      <span className="voice-wave-symbol" aria-hidden="true">
        {transcribing ? <LoaderCircle size={18} /> : <Mic size={18} />}
      </span>
      <span className="voice-wave-label">
        <i className="voice-wave-dot" aria-hidden="true" />
        {transcribing ? t('Распознаём речь…') : t('Говорите…')}
      </span>
      <canvas ref={canvas} aria-hidden="true" />
      <span className="voice-wave-time">{time}</span>
    </div>
  )
}
