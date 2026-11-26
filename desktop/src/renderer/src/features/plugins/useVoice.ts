import { useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
import type { AsrEvent, LiveTranscript } from '../../../../shared/stt'
import { useI18n } from '../../i18n'
import { ensureSttReady } from '../voice/sttReady'
import { bytesToBase64, openPcmStream, type PcmStreamHandle } from '../voice/pcmStream'

const EMPTY_TRANSCRIPT: LiveTranscript = { committed: '', open: '', tail: '' }

export function formatLiveTranscript(transcript: LiveTranscript): string {
  return [transcript.committed, transcript.open, transcript.tail]
    .map((part) => part.trim())
    .filter(Boolean)
    .join(' ')
    .trim()
}

export function useVoice(workspace: Workspace, onText: (text: string) => void) {
  const { t, language: uiLanguage } = useI18n()
  const [recording, setRecording] = useState(false)
  const [transcribing, setTranscribing] = useState(false)
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null)
  const [liveTranscript, setLiveTranscript] = useState<LiveTranscript>(EMPTY_TRANSCRIPT)

  const streamRef = useRef<PcmStreamHandle | null>(null)
  const sessionIdRef = useRef<string | null>(null)
  const cancelledRef = useRef(false)
  const liveRef = useRef<LiveTranscript>(EMPTY_TRANSCRIPT)
  const startingRef = useRef(false)
  const finishingRef = useRef(false)
  const onTextRef = useRef(onText)
  onTextRef.current = onText

  useEffect(() => {
    return window.prime.onEvent((event) => {
      if (!('type' in event) || event.type !== 'asr') return
      const payload = event.payload as AsrEvent
      const activeId = sessionIdRef.current
      if (!activeId || !('sessionId' in payload) || payload.sessionId !== activeId) return

      if (payload.type === 'partial') {
        const next: LiveTranscript = {
          committed: payload.committed || '',
          open: payload.open || '',
          tail: payload.tail || ''
        }
        liveRef.current = next
        setLiveTranscript(next)
      } else if (payload.type === 'commit') {
        const chunk = (payload.text || '').trim()
        if (!chunk) return
        const next: LiveTranscript = {
          committed: [liveRef.current.committed, chunk].filter(Boolean).join(' '),
          open: '',
          tail: ''
        }
        liveRef.current = next
        setLiveTranscript(next)
      } else if (payload.type === 'final') {
        const finalText = (payload.text || '').trim()
        if (finalText) {
          const next: LiveTranscript = { committed: finalText, open: '', tail: '' }
          liveRef.current = next
          setLiveTranscript(next)
        }
      } else if (payload.type === 'error' && payload.message) {
        workspace.errorHandler(new Error(payload.message))
      }
    })
  }, [workspace])

  async function releaseAudio() {
    const activeStream = streamRef.current
    streamRef.current = null
    setAnalyser(null)
    if (activeStream) {
      await activeStream.close().catch(() => {})
    }
  }

  function resetLive() {
    liveRef.current = EMPTY_TRANSCRIPT
    setLiveTranscript(EMPTY_TRANSCRIPT)
  }

  async function start() {
    if (startingRef.current || streamRef.current || finishingRef.current) return
    startingRef.current = true
    cancelledRef.current = false
    resetLive()
    try {
      const asrLanguage = await ensureSttReady(workspace.call, t)
      if (cancelledRef.current) return

      const started = await window.prime.asr({
        action: 'start',
        language: asrLanguage,
        startLanguage: uiLanguage
      })
      const sessionId = String(started?.sessionId || '')
      if (!sessionId) {
        throw new Error(t('Не удалось запустить локальное распознавание речи'))
      }
      if (cancelledRef.current) {
        void window.prime.asr({ action: 'cancel', sessionId }).catch(() => {})
        return
      }
      sessionIdRef.current = sessionId

      let pushQueue = Promise.resolve()
      const handle = await openPcmStream({
        deviceId: workspace.preferences.micDeviceId || undefined,
        onChunk: (pcmS16le) => {
          const currentId = sessionIdRef.current
          if (!currentId || cancelledRef.current) return
          const pcmBase64 = bytesToBase64(pcmS16le)
          pushQueue = pushQueue
            .then(() => window.prime.asr({ action: 'push', sessionId: currentId, pcmBase64 }))
            .catch(() => {})
        }
      })

      if (cancelledRef.current) {
        await handle.close().catch(() => {})
        void window.prime.asr({ action: 'cancel', sessionId }).catch(() => {})
        sessionIdRef.current = null
        return
      }

      streamRef.current = handle
      setAnalyser(handle.analyser)
      setRecording(true)
    } catch (error) {
      const staleId = sessionIdRef.current
      sessionIdRef.current = null
      if (staleId) {
        void window.prime.asr({ action: 'cancel', sessionId: staleId }).catch(() => {})
      }
      await releaseAudio()
      setRecording(false)
      resetLive()
      workspace.errorHandler(error)
    } finally {
      startingRef.current = false
    }
  }

  async function finish(): Promise<string> {
    if (finishingRef.current) return ''
    const sessionId = sessionIdRef.current
    if (!sessionId && !streamRef.current) return ''

    finishingRef.current = true
    setRecording(false)
    setTranscribing(true)
    try {
      await releaseAudio()
      if (cancelledRef.current || !sessionId) return ''
      const result = await window.prime.asr({ action: 'finish', sessionId })
      const finalText = String(result?.text || '').trim() || formatLiveTranscript(liveRef.current)
      return finalText
    } catch (error) {
      workspace.errorHandler(error)
      return formatLiveTranscript(liveRef.current)
    } finally {
      if (sessionIdRef.current === sessionId) {
        sessionIdRef.current = null
      }
      setTranscribing(false)
      resetLive()
      finishingRef.current = false
    }
  }

  async function toggle() {
    if (recording) {
      const text = await finish()
      if (text) onTextRef.current(text)
      return
    }
    if (transcribing) return
    await start()
  }

  async function cancel() {
    cancelledRef.current = true
    const sessionId = sessionIdRef.current
    sessionIdRef.current = null
    setRecording(false)
    setTranscribing(false)
    resetLive()
    await releaseAudio()
    if (sessionId) {
      await window.prime.asr({ action: 'cancel', sessionId }).catch(() => {})
    }
  }

  useEffect(
    () => () => {
      cancelledRef.current = true
      const sessionId = sessionIdRef.current
      sessionIdRef.current = null
      void streamRef.current?.close().catch(() => {})
      if (sessionId) {
        void window.prime.asr({ action: 'cancel', sessionId }).catch(() => {})
      }
    },
    []
  )

  return { recording, transcribing, analyser, liveTranscript, toggle, finish, cancel }
}
