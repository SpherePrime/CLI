import { useEffect, useRef } from 'react'
import { Mic, MicOff, PhoneOff, VolumeX } from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Conversation } from './useConversation'
import '../../styles/conversation.css'

const labels = {
  off: '',
  preparing: 'Подготовка голоса…',
  listening: 'Слушаю',
  hearing: 'Говорите…',
  transcribing: 'Распознаю…',
  waiting: 'Prime думает…',
  speaking: 'Prime отвечает'
} as const

export function ConversationBar({ conversation }: { conversation: Conversation }) {
  const { t } = useI18n()
  const canvas = useRef<HTMLCanvasElement>(null)
  const { state, analyser, muted } = conversation
  useEffect(() => {
    const element = canvas.current
    if (!element) return
    const context = element.getContext('2d')!
    const data = new Uint8Array(analyser?.frequencyBinCount ?? 32)
    let frame = 0
    let phase = 0
    const draw = () => {
      frame = requestAnimationFrame(draw)
      phase += 0.05
      const { width, height } = element
      context.clearRect(0, 0, width, height)
      if (analyser && !muted) analyser.getByteFrequencyData(data)
      const color = getComputedStyle(element).getPropertyValue('--accent').trim() || '#ff7a45'
      context.fillStyle = color
      const bars = 24
      for (let index = 0; index < bars; index++) {
        const live = analyser && !muted ? data[Math.floor((index / bars) * data.length * 0.6)] / 255 : 0
        const idle =
          state === 'speaking' || state === 'waiting' || state === 'transcribing'
            ? 0.25 + 0.2 * Math.sin(phase * (state === 'speaking' ? 3 : 1.5) + index * 0.6)
            : 0.08
        const value = Math.max(live, idle)
        const bar = Math.max(3, value * height)
        const x = (index / bars) * width + 1
        context.globalAlpha = 0.45 + value * 0.55
        context.fillRect(x, (height - bar) / 2, width / bars - 3, bar)
      }
    }
    draw()
    return () => cancelAnimationFrame(frame)
  }, [analyser, muted, state])
  const progress = conversation.preparing
  return (
    <div className={`conversation-bar is-${state}`} role="region" aria-label={t('Голосовой разговор')}>
      <canvas ref={canvas} width={168} height={34} aria-hidden="true" />
      <div className="conversation-status" role="status" aria-live="polite">
        <strong>
          {state === 'preparing' && progress?.state === 'downloading'
            ? t('Загрузка голосовой модели · {value0}%', { value0: Math.floor(progress.percent ?? 0) })
            : muted
              ? t('Микрофон выключен')
              : t(labels[state])}
        </strong>
        {conversation.heard && <span title={conversation.heard}>{conversation.heard}</span>}
      </div>
      <div className="conversation-actions">
        {state === 'speaking' && (
          <button className="icon small" aria-label={t('Остановить озвучку')} title={t('Остановить озвучку')} onClick={conversation.stopSpeaking}>
            <VolumeX size={15} />
          </button>
        )}
        <button
          className={`icon small ${muted ? 'active' : ''}`}
          aria-label={muted ? t('Включить микрофон') : t('Выключить микрофон')}
          title={muted ? t('Включить микрофон') : t('Выключить микрофон')}
          aria-pressed={muted}
          disabled={state === 'preparing'}
          onClick={conversation.toggleMute}
        >
          {muted ? <MicOff size={15} /> : <Mic size={15} />}
        </button>
        <button className="conversation-end" onClick={() => void conversation.stop()}>
          <PhoneOff size={14} />
          {t('Завершить')}
        </button>
      </div>
    </div>
  )
}
