import { useCallback, useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import type { Message } from '../../../../shared/contracts'
import type { SpeechStatus } from '../../../../shared/speech'
import { useI18n } from '../../i18n'
import { Listener } from './listener'
import { Speaker } from './speaker'
import { speakableText, speechChunks } from './speechText'
import { resample, wavBase64 } from './wav'
import { isCancelAnswer, parseAnswer, questionSpeech, type VoiceAnswer, type VoiceQuestion } from './voiceQuestions'

type Asking = { batchId: string; questions: VoiceQuestion[]; index: number; responses: VoiceAnswer[] }

export type ConversationState =
  'off' | 'preparing' | 'listening' | 'hearing' | 'transcribing' | 'waiting' | 'speaking'

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
  const [outputAnalyser, setOutputAnalyser] = useState<AnalyserNode | null>(null)
  const [startedAt, setStartedAt] = useState<number | null>(null)
  const listener = useRef<Listener>(undefined)
  const speaker = useRef<Speaker>(undefined)
  const pending = useRef<{ from: number; sawBusy: boolean } | undefined>(undefined)
  const asking = useRef<Asking | undefined>(undefined)
  const askedBatches = useRef(new Set<string>())
  const [question, setQuestion] = useState('')
  const generation = useRef(0)
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
    generation.current++
    stateRef.current = 'off'
    pending.current = undefined
    asking.current = undefined
    askedBatches.current.clear()
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
  }, [])

  useEffect(() => () => void stop(), [stop])
  useEffect(() => {
    if (stateRef.current !== 'off') void stop()
  }, [w.project?.id, stop])

  const askCurrent = useCallback(() => {
    const current = asking.current
    if (!current) return
    const text = questionSpeech(current.questions[current.index], current.index, current.questions.length, t)
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
        await workspace.call('answer', { batch_request_id: current.batchId, responses: current.responses })
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
    const batches: any[] = w.questions ?? []
    if (asking.current && !batches.some((batch) => batch.id === asking.current?.batchId)) {
      asking.current = undefined
      setQuestion('')
      speaker.current?.stop()
    }
    if (asking.current) return
    const next = batches.find((batch) => batch?.id && batch.questions?.length && !askedBatches.current.has(batch.id))
    if (!next) return
    askedBatches.current.add(next.id)
    asking.current = { batchId: next.id, questions: next.questions, index: 0, responses: [] }
    askCurrent()
  }, [w.questions, askCurrent])

  const handleSegment = useCallback(async (samples: Float32Array, sampleRate: number) => {
    if (!listener.current) return
    const turnGeneration = generation.current
    setState('transcribing')
    try {
      const audio = wavBase64(await resample(samples, sampleRate))
      if (turnGeneration !== generation.current) return
      const result = await latest.current.call('voice', {
        audio_base64: audio,
        mime_type: 'audio/wav'
      })
      const text = String(result?.text || '').trim()
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
  }, [answerQuestion])

  const start = useCallback(async () => {
    if (stateRef.current !== 'off') return
    const callGeneration = ++generation.current
    stateRef.current = 'preparing'
    setState('preparing')
    setStartedAt(Date.now())
    try {
      const plugins = await latest.current.call('plugins')
      if (callGeneration !== generation.current) return
      const list = Array.isArray(plugins) ? plugins : plugins?.plugins || []
      if (!list.find((plugin: any) => plugin.name === 'voice')?.enabled)
        throw new Error(t('Включите плагин Voice в разделе «Плагины»'))
      const status: SpeechStatus = await window.prime.speech({ action: 'prepare' })
      if (callGeneration !== generation.current) return
      if (status.state !== 'ready')
        throw new Error(status.error || t('Голосовая модель недоступна'))
      speaker.current = new Speaker({
        voice,
        speed,
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
        sensitivity: () => (speaker.current?.speaking ? 2.2 : 1),
        onSpeechStart: () => {
          if (speaker.current?.speaking) speaker.current.stop()
          setState('hearing')
        },
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
    },
    stopSpeaking: () => {
      speaker.current?.stop()
      if (listener.current) setState(pending.current ? 'waiting' : 'listening')
    }
  }
}

export type Conversation = ReturnType<typeof useConversation>
