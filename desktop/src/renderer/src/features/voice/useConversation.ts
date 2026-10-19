import { useCallback, useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import type { Message } from '../../../../shared/contracts'
import type { SpeechStatus } from '../../../../shared/speech'
import { useI18n } from '../../i18n'
import { Listener } from './listener'
import { Speaker } from './speaker'
import { speakableText, speechChunks } from './speechText'
import { resample, wavBase64 } from './wav'

export type ConversationState = 'off' | 'preparing' | 'listening' | 'hearing' | 'transcribing' | 'waiting' | 'speaking'

function assistantText(messages: Message[], after: number) {
  return messages
    .slice(after)
    .filter((message) => message.role === 'assistant')
    .flatMap((message) => message.parts)
    .filter((part) => part.type === 'text' && !part.data?.hidden && part.data?.text)
    .map((part) => String(part.data.text))
    .join('\n')
}

export function useConversation(w: Workspace) {
  const { t } = useI18n()
  const [state, setState] = useState<ConversationState>('off')
  const [muted, setMuted] = useState(false)
  const [preparing, setPreparing] = useState<SpeechStatus>()
  const [heard, setHeard] = useState('')
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null)
  const listener = useRef<Listener>(undefined)
  const speaker = useRef<Speaker>(undefined)
  const pending = useRef<{ from: number; sawBusy: boolean } | undefined>(undefined)
  const latest = useRef(w)
  latest.current = w
  const stateRef = useRef(state)
  stateRef.current = state
  const voice = w.preferences.speechVoice ?? 'F1'
  const speed = w.preferences.speechSpeed ?? 1.05

  useEffect(() => {
    speaker.current?.update({ voice, speed })
  }, [voice, speed])

  const stop = useCallback(async () => {
    pending.current = undefined
    const currentListener = listener.current
    const currentSpeaker = speaker.current
    listener.current = undefined
    speaker.current = undefined
    setAnalyser(null)
    setState('off')
    setHeard('')
    setMuted(false)
    await currentListener?.stop()
    await currentSpeaker?.dispose()
  }, [])

  useEffect(() => () => void stop(), [stop])
  useEffect(() => {
    if (stateRef.current !== 'off') void stop()
  }, [w.project?.id, stop])

  const handleSegment = useCallback(
    async (samples: Float32Array, sampleRate: number) => {
      if (!listener.current) return
      setState('transcribing')
      try {
        const audio = wavBase64(await resample(samples, sampleRate))
        const result = await latest.current.call('voice', { audio_base64: audio, mime_type: 'audio/wav' })
        const text = String(result?.text || '').trim()
        if (!listener.current) return
        if (!text) {
          setState(pending.current ? 'waiting' : 'listening')
          return
        }
        setHeard(text)
        pending.current = { from: latest.current.messages.length, sawBusy: false }
        setState('waiting')
        const sent = await latest.current.send(text)
        if (!sent) {
          pending.current = undefined
          setState('listening')
        }
      } catch (error) {
        latest.current.errorHandler(error)
        if (listener.current) setState('listening')
      }
    },
    []
  )

  const start = useCallback(async () => {
    if (stateRef.current !== 'off') return
    setState('preparing')
    try {
      const plugins = await latest.current.call('plugins')
      const list = Array.isArray(plugins) ? plugins : plugins?.plugins || []
      if (!list.find((plugin: any) => plugin.name === 'voice')?.enabled)
        throw new Error(t('Включите плагин Voice в разделе «Плагины»'))
      const status: SpeechStatus = await window.prime.speech({ action: 'prepare' })
      if (status.state !== 'ready') throw new Error(status.error || t('Голосовая модель недоступна'))
      speaker.current = new Speaker({
        voice,
        speed,
        onState: (speaking) => {
          if (!listener.current) return
          setState((current) => (speaking ? 'speaking' : current === 'speaking' ? 'listening' : current))
        },
        onError: (error) => latest.current.errorHandler(error)
      })
      const instance = new Listener({
        silenceMs: 900,
        sensitivity: () => (speaker.current?.speaking ? 2.2 : 1),
        onSpeechStart: () => {
          if (speaker.current?.speaking) speaker.current.stop()
          setState('hearing')
        },
        onSegment: (samples, sampleRate) => void handleSegment(samples, sampleRate)
      })
      await instance.start()
      listener.current = instance
      setAnalyser(instance.analyser ?? null)
      setState('listening')
    } catch (error) {
      await stop()
      latest.current.errorHandler(error)
    }
  }, [handleSegment, stop, t, voice, speed])

  useEffect(() => {
    return window.prime.onEvent((event) => {
      if (event.type === 'speech') setPreparing(event.payload as SpeechStatus)
    })
  }, [])

  useEffect(() => {
    const turn = pending.current
    if (!turn || !listener.current) return
    if (w.busy) {
      turn.sawBusy = true
      return
    }
    if (!turn.sawBusy && w.messages.length <= turn.from) return
    const text = speakableText(assistantText(w.messages, turn.from), t('Код показан в чате'))
    if (!text && !turn.sawBusy) return
    pending.current = undefined
    if (stateRef.current === 'hearing' || stateRef.current === 'transcribing') return
    if (!text) {
      setState('listening')
      return
    }
    void speaker.current?.speak(speechChunks(text))
  }, [w.busy, w.messages, t])

  return {
    state,
    muted,
    heard,
    analyser,
    preparing,
    start,
    stop,
    toggleMute: () => {
      const next = !muted
      setMuted(next)
      listener.current?.setMuted(next)
    },
    stopSpeaking: () => {
      speaker.current?.stop()
      if (listener.current) setState(pending.current ? 'waiting' : 'listening')
    }
  }
}

export type Conversation = ReturnType<typeof useConversation>
