import { useEffect, useRef, useState } from 'react'
import {
  ArrowUpRight,
  AudioLines,
  MessageSquare,
  Mic,
  MicOff,
  PhoneOff,
  VolumeX
} from 'lucide-react'
import { useI18n } from '../../i18n'
import type { Conversation } from './useConversation'
import { VoiceOrb } from './VoiceOrb'
import '../../styles/voice-call.css'

const labels = {
  off: 'Голосовой разговор',
  preparing: 'Подготовка голоса…',
  listening: 'Слушаю',
  hearing: 'Говорите…',
  transcribing: 'Распознаю…',
  waiting: 'Prime думает…',
  speaking: 'Prime отвечает'
} as const

function useCallTime(startedAt: number | null) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    setNow(Date.now())
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [startedAt])
  const seconds = startedAt ? Math.max(0, Math.floor((now - startedAt) / 1000)) : 0
  return `${Math.floor(seconds / 60)
    .toString()
    .padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`
}

function useCallStatus(conversation: Conversation) {
  const { t } = useI18n()
  if (conversation.state === 'preparing' && conversation.preparing?.state === 'downloading') {
    return t('Загрузка голосовой модели · {value0}%', {
      value0: Math.floor(conversation.preparing.percent ?? 0)
    })
  }
  if (
    conversation.muted &&
    (conversation.state === 'listening' || conversation.state === 'hearing')
  )
    return t('Микрофон выключен')
  return t(labels[conversation.state])
}

function MuteButton({
  conversation,
  compact = false
}: {
  conversation: Conversation
  compact?: boolean
}) {
  const { t } = useI18n()
  const label = conversation.muted ? t('Включить микрофон') : t('Выключить микрофон')
  return (
    <button
      type="button"
      className={`voice-call-mute${conversation.muted ? ' is-muted' : ''}`}
      title={label}
      aria-label={label}
      aria-pressed={conversation.muted}
      disabled={conversation.state === 'preparing'}
      onClick={conversation.toggleMute}
    >
      {conversation.muted ? <MicOff size={compact ? 17 : 22} /> : <Mic size={compact ? 17 : 22} />}
    </button>
  )
}

export function ConversationCall({
  conversation,
  onOpenChat
}: {
  conversation: Conversation
  onOpenChat(): void
}) {
  const { t } = useI18n()
  const elapsed = useCallTime(conversation.startedAt)
  const status = useCallStatus(conversation)
  const panel = useRef<HTMLElement>(null)
  useEffect(() => {
    const previous = document.activeElement
    panel.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
    return () => {
      queueMicrotask(() => {
        if (
          previous instanceof HTMLElement &&
          previous.isConnected &&
          !previous.closest('[inert]') &&
          !previous.matches(':disabled')
        )
          previous.focus()
      })
    }
  }, [])
  const hint =
    conversation.state === 'preparing'
      ? 'Первый запуск может занять немного времени'
      : conversation.state === 'speaking'
        ? 'Можете перебить меня голосом'
        : conversation.muted
          ? 'Включите микрофон, чтобы продолжить'
          : conversation.state === 'waiting' || conversation.state === 'transcribing'
            ? 'Разговор продолжается автоматически'
            : 'Говорите свободно. Я отвечу, когда вы закончите.'
  return (
    <section
      ref={panel}
      className="voice-call"
      aria-label={t('Голосовой разговор')}
      onKeyDown={(event) => {
        if (event.key === 'Escape') {
          event.preventDefault()
          event.stopPropagation()
          onOpenChat()
        }
        if (event.key !== 'Tab') return
        const buttons = Array.from(
          event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')
        )
        if (!buttons.length) return
        event.preventDefault()
        const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
        const next =
          index < 0
            ? event.shiftKey
              ? buttons.length - 1
              : 0
            : (index + (event.shiftKey ? -1 : 1) + buttons.length) % buttons.length
        buttons[next].focus()
      }}
    >
      <header className="voice-call-header">
        <div className="voice-call-session">
          <span className="voice-call-dot" />
          <span>{t('Голосовой разговор')}</span>
          <time>{elapsed}</time>
        </div>
        <button type="button" className="voice-call-chat" onClick={onOpenChat}>
          <MessageSquare size={16} />
          {t('Открыть чат')}
          <ArrowUpRight size={14} />
        </button>
      </header>
      <div className="voice-call-stage">
        <div className="voice-call-identity">PRIME</div>
        <VoiceOrb
          state={conversation.state}
          muted={conversation.muted}
          analyser={conversation.analyser}
          outputAnalyser={conversation.outputAnalyser}
        />
        <div className="voice-call-copy">
          <h2 role="status" aria-live="polite" aria-atomic="true">
            {status}
          </h2>
          <p>{t(hint)}</p>
        </div>
        <div className="voice-call-transcript">
          {conversation.heard && <p title={conversation.heard}>{conversation.heard}</p>}
        </div>
      </div>
      <footer className="voice-call-footer">
        <div className="voice-call-controls">
          <div className="voice-call-control">
            <MuteButton conversation={conversation} />
            <span>{conversation.muted ? t('Включить') : t('Микрофон')}</span>
          </div>
          <div className="voice-call-control">
            <button
              type="button"
              className="voice-call-end"
              title={t('Завершить')}
              aria-label={t('Завершить')}
              onClick={() => void conversation.stop()}
            >
              <PhoneOff size={24} />
            </button>
            <span>{t('Завершить')}</span>
          </div>
          <div className="voice-call-control">
            <button
              type="button"
              className="voice-call-interrupt"
              title={t('Остановить озвучку')}
              aria-label={t('Остановить озвучку')}
              disabled={conversation.state !== 'speaking'}
              onClick={conversation.stopSpeaking}
            >
              <VolumeX size={22} />
            </button>
            <span>{t('Прервать')}</span>
          </div>
        </div>
        <p>{t('История разговора сохраняется в чате')}</p>
      </footer>
    </section>
  )
}

export function ConversationCallPill({
  conversation,
  onReturnToCall
}: {
  conversation: Conversation
  onReturnToCall(): void
}) {
  const { t } = useI18n()
  const elapsed = useCallTime(conversation.startedAt)
  const status = useCallStatus(conversation)
  return (
    <div className="voice-call-pill" role="region" aria-label={t('Голосовой разговор')}>
      <button
        type="button"
        className="voice-call-return"
        title={t('Вернуться к разговору')}
        aria-label={t('Вернуться к разговору')}
        onClick={onReturnToCall}
      >
        <AudioLines size={20} />
        <span>
          <strong>{status}</strong>
          <small>
            {t('Вернуться к разговору')} · {elapsed}
          </small>
        </span>
        <ArrowUpRight size={15} />
      </button>
      <MuteButton conversation={conversation} compact />
      <button
        type="button"
        className="voice-call-end"
        title={t('Завершить')}
        aria-label={t('Завершить')}
        onClick={() => void conversation.stop()}
      >
        <PhoneOff size={18} />
      </button>
    </div>
  )
}
