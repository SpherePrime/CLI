import { useCallback, useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import type { Message } from '../../../../shared/contracts'
import type { SpeechStatus } from '../../../../shared/speech'
import type { AsrEvent, AsrLanguage } from '../../../../shared/stt'
import { useI18n } from '../../i18n'
import { Listener } from './listener'
import { Speaker } from './speaker'
import { speakableText, speechChunks } from './speechText'
import { ensureSttReady } from './sttReady'
import { bytesToBase64, downsampleTo16k, float32ToS16LE } from './pcmStream'
import {
  isCancelAnswer,
  parseAnswer,
  questionSpeech,
  type VoiceAnswer,
  type VoiceQuestion
} from './voiceQuestions'

type Asking = {
  batchId: string
  sessionId?: string
  questions: VoiceQuestion[]
  index: number
  responses: VoiceAnswer[]
}

export type ConversationState =
  | 'off'
  | 'preparing'
  | 'listening'
  | 'hearing'
  | 'transcribing'
  | 'waiting'
  | 'speaking'

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
  const { t, language: uiLanguage } = useI18n()
  const [state, setState] = useState<ConversationState>('off')
  const [muted, setMuted] = useState(false)
  const [preparing, setPreparing] = useState<SpeechStatus>()
  const [heard, setHeard] = useState('')
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null)
  const [outputAnalyser, setOutputAnalyser] = useState<AnalyserNode | null>(null)
  const [startedAt, setStartedAt] = useState<number | null>(null)
  const listener = useRef<Listener>(undefined)
  const speaker = useRef<Speaker>(undefined)
  const pending = useRef<{ from: number; sawBusy: boolean } | undefined>(undefined)
  const asking = useRef<Asking | undefined>(undefined)
  const askedBatches = useRef(new Set<string>())
  const [question, setQuestion] = useState('')
  const generation = useRef(0)
  const asrLanguageRef = useRef<AsrLanguage>('auto')
  const liveSessionIdRef = useRef<string | null>(null)
  const liveStartPromiseRef = useRef<Promise<string | null> | null>(null)
  const livePushQueueRef = useRef<Promise<unknown>>(Promise.resolve())
  const latest = useRef(w)
  latest.current = w
  const stateRef = useRef(state)
  stateRef.current = state
  const voice = w.preferences.speechVoice ?? 'F1'
  const speed = w.preferences.speechSpeed ?? 1.05

  useEffect(() => {
    speaker.current?.update({ voice, speed })
  }, [voice, speed])

  const cancelLiveAsr = useCallback(() => {
    const id = liveSessionIdRef.current
    liveSessionIdRef.current = null
    liveStartPromiseRef.current = null
    if (id) {
      void window.prime.asr({ action: 'cancel', sessionId: id }).catch(() => {})
    }
  }, [])

  const stop = useCallback(async () => {
    generation.current++
    stateRef.current = 'off'
    pending.current = undefined
    asking.current = undefined
    askedBatches.current.clear()
    cancelLiveAsr()
    setQuestion('')
    const currentListener = listener.current
    const currentSpeaker = speaker.current
    listener.current = undefined
    speaker.current = undefined
    setAnalyser(null)
    setOutputAnalyser(null)
    setStartedAt(null)
    setState('off')
    setHeard('')
    setMuted(false)
    await currentListener?.stop()
    await currentSpeaker?.dispose()
  }, [cancelLiveAsr])

  useEffect(() => () => void stop(), [stop])
  useEffect(() => {
    if (stateRef.current !== 'off') void stop()
  }, [w.project?.id, stop])

  const askCurrent = useCallback(() => {
    const current = asking.current
    if (!current) return
    const text = questionSpeech(
      current.questions[current.index],
      current.index,
      current.questions.length,
      t
    )
    setQuestion(current.questions[current.index].question)
    void speaker.current?.speak(speechChunks(speakableText(text, t('Код показан в чате'))))
  }, [t])

  const answerQuestion = useCallback(
    async (text: string) => {
      const current = asking.current
      if (!current) return
      const workspace = latest.current
      try {
        if (isCancelAnswer(text)) {
          asking.current = undefined
          setQuestion('')
          await workspace.call('cancelQuestion', { id: current.batchId })
          setState(pending.current ? 'waiting' : 'listening')
          return
        }
        current.responses.push(parseAnswer(current.questions[current.index], text))
        if (current.index + 1 < current.questions.length) {
          current.index++
          askCurrent()
          return
        }
        asking.current = undefined
        setQuestion('')
        setState('waiting')
        await workspace.call('answer', {
          session_id: current.sessionId ?? workspace.session?.id,
          batch_request_id: current.batchId,
          responses: current.responses
        })
      } catch (error) {
        asking.current = undefined
        setQuestion('')
        workspace.errorHandler(error)
        if (listener.current) setState('listening')
      }
    },
    [askCurrent]
  )

  useEffect(() => {
    if (!listener.current || stateRef.current === 'off') return
    const activeSessionId = latest.current.session?.id
    const batches: any[] = activeSessionId
      ? (w.questions ?? []).filter((batch) => batch?.session_id === activeSessionId)
      : []
    if (asking.current && !batches.some((batch) => batch.id === asking.current?.batchId)) {
      asking.current = undefined
      setQuestion('')
      speaker.current?.stop()
    }
    if (asking.current) return
    const next = batches.find(
      (batch) => batch?.id && batch.questions?.length && !askedBatches.current.has(batch.id)
    )
    if (!next) return
    askedBatches.current.add(next.id)
    asking.current = {
      batchId: next.id,
      sessionId: next.session_id || activeSessionId,
      questions: next.questions,
      index: 0,
      responses: []
    }
    askCurrent()
  }, [w.questions, w.session?.id, askCurrent])

  const beginTurnStream = useCallback(() => {
    const turnGeneration = generation.current
    cancelLiveAsr()
    const startPromise = window.prime
      .asr({
        action: 'start',
        language: asrLanguageRef.current,
        startLanguage: uiLanguage
      })
      .then((res) => {
        const id = String(res?.sessionId || '')
        if (!id || turnGeneration !== generation.current) {
          if (id) void window.prime.asr({ action: 'cancel', sessionId: id }).catch(() => {})
          return null
        }
        liveSessionIdRef.current = id
        return id
      })
      .catch(() => null)
    liveStartPromiseRef.current = startPromise
    livePushQueueRef.current = startPromise
  }, [cancelLiveAsr, uiLanguage])

  const pushTurnChunk = useCallback((samples: Float32Array, sampleRate: number) => {
    const downsampled = downsampleTo16k(samples, sampleRate)
    if (!downsampled.length) return
    const pcmBase64 = bytesToBase64(float32ToS16LE(downsampled))
    const startPromise = liveStartPromiseRef.current
    if (!startPromise) return
    livePushQueueRef.current = livePushQueueRef.current
      .then(async () => {
        const sessionId = liveSessionIdRef.current || (await startPromise)
        if (!sessionId) return
        await window.prime.asr({ action: 'push', sessionId, pcmBase64 })
      })
      .catch(() => {})
  }, [])

  const handleSegment = useCallback(
    async (samples: Float32Array, sampleRate: number) => {
      if (!listener.current) return
      const turnGeneration = generation.current
      setState('transcribing')
      try {
        await livePushQueueRef.current.catch(() => {})
        const sessionId = liveSessionIdRef.current
        liveSessionIdRef.current = null
        liveStartPromiseRef.current = null

        let text = ''
        if (sessionId) {
          const result = await window.prime.asr({ action: 'finish', sessionId })
          text = String(result?.text || '').trim()
        } else {
          const pcmBase64 = bytesToBase64(float32ToS16LE(downsampleTo16k(samples, sampleRate)))
          const result = await window.prime.asr({
            action: 'transcribe',
            pcmBase64,
            language: asrLanguageRef.current,
            startLanguage: uiLanguage
          })
          text = String(result?.text || '').trim()
        }

        if (!listener.current || turnGeneration !== generation.current) return
        if (!text) {
          setState(pending.current ? 'waiting' : 'listening')
          return
        }
        setHeard(text)
        if (asking.current) {
          await answerQuestion(text)
          return
        }
        pending.current = { from: latest.current.messages.length, sawBusy: false }
        setState('waiting')
        const sent = await latest.current.send(text)
        if (turnGeneration !== generation.current) return
        if (!sent) {
          pending.current = undefined
          setState('listening')
        }
      } catch (error) {
        if (turnGeneration !== generation.current) return
        latest.current.errorHandler(error)
        if (listener.current) setState('listening')
      }
    },
    [answerQuestion, uiLanguage]
  )

  const start = useCallback(async () => {
    if (stateRef.current !== 'off') return
    const callGeneration = ++generation.current
    stateRef.current = 'preparing'
    setState('preparing')
    setStartedAt(Date.now())
    try {
      asrLanguageRef.current = await ensureSttReady(latest.current.call, t)
      if (callGeneration !== generation.current) return
      const status: SpeechStatus = await window.prime.speech({ action: 'prepare' })
      if (callGeneration !== generation.current) return
      if (status.state !== 'ready')
        throw new Error(status.error || t('Голосовая модель недоступна'))
      speaker.current = new Speaker({
        voice,
        speed,
        deviceId: latest.current.preferences.speakerDeviceId || undefined,
        onState: (speaking) => {
          if (!listener.current) return
          setState((current) =>
            speaking ? 'speaking' : current === 'speaking' ? 'listening' : current
          )
        },
        onError: (error) => latest.current.errorHandler(error)
      })
      setOutputAnalyser(speaker.current.analyser)
      const instance = new Listener({
        silenceMs: 900,
        deviceId: latest.current.preferences.micDeviceId || undefined,
        sensitivity: () => (speaker.current?.speaking ? 2.2 : 1),
        onSpeechStart: () => {
          if (speaker.current?.speaking) speaker.current.stop()
          setState('hearing')
          beginTurnStream()
        },
        onSpeechChunk: (samples, sampleRate) => pushTurnChunk(samples, sampleRate),
        onSegment: (samples, sampleRate) => void handleSegment(samples, sampleRate)
      })
      listener.current = instance
      await instance.start()
      if (callGeneration !== generation.current) return
      setAnalyser(instance.analyser ?? null)
      setState('listening')
    } catch (error) {
      if (callGeneration !== generation.current) return
      await stop()
      latest.current.errorHandler(error)
    }
  }, [beginTurnStream, handleSegment, pushTurnChunk, stop, t, voice, speed])

  useEffect(() => {
    return window.prime.onEvent((event) => {
      if (!('type' in event)) return
      if (event.type === 'speech') {
        setPreparing(event.payload as SpeechStatus)
        return
      }
      if (event.type === 'asr') {
        const payload = event.payload as AsrEvent
        if (
          payload.type === 'partial' &&
          liveSessionIdRef.current &&
          payload.sessionId === liveSessionIdRef.current
        ) {
          const live = [payload.committed, payload.open, payload.tail]
            .map((part) => part.trim())
            .filter(Boolean)
            .join(' ')
          if (live) setHeard(live)
        }
      }
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
    question,
    analyser,
    outputAnalyser,
    startedAt,
    preparing,
    start,
    stop,
    toggleMute: () => {
      const next = !muted
      setMuted(next)
      listener.current?.setMuted(next)
      if (next) cancelLiveAsr()
    },
    stopSpeaking: () => {
      speaker.current?.stop()
      if (listener.current) setState(pending.current ? 'waiting' : 'listening')
    }
  }
}

export type Conversation = ReturnType<typeof useConversation>
