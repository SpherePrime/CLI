import { useI18n } from '../../i18n'
import { useEffect, useRef, useState } from 'react'
import type { Workspace } from '../../app/useWorkspace'
function wav(samples: Float32Array): string {
  const buffer = new ArrayBuffer(44 + samples.length * 2)
  const data = new DataView(buffer)
  const text = (at: number, value: string) => {
    for (let i = 0; i < value.length; i++) data.setUint8(at + i, value.charCodeAt(i))
  }
  text(0, 'RIFF')
  data.setUint32(4, buffer.byteLength - 8, true)
  text(8, 'WAVE')
  text(12, 'fmt ')
  data.setUint32(16, 16, true)
  data.setUint16(20, 1, true)
  data.setUint16(22, 1, true)
  data.setUint32(24, 16000, true)
  data.setUint32(28, 32000, true)
  data.setUint16(32, 2, true)
  data.setUint16(34, 16, true)
  text(36, 'data')
  data.setUint32(40, samples.length * 2, true)
  for (let i = 0; i < samples.length; i++)
    data.setInt16(44 + i * 2, Math.max(-1, Math.min(1, samples[i])) * 32767, true)
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let start = 0; start < bytes.length; start += 8192)
    binary += String.fromCharCode(...bytes.subarray(start, start + 8192))
  return btoa(binary)
}
export function useVoice(workspace: Workspace, onText: (text: string) => void) {
  const { t } = useI18n()
  const [recording, setRecording] = useState(false)
  const [transcribing, setTranscribing] = useState(false)
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null)
  const recorder = useRef<MediaRecorder | null>(null)
  const stream = useRef<MediaStream | null>(null)
  const meter = useRef<AudioContext | null>(null)
  const discard = useRef(false)
  const pending = useRef<((text: string) => void) | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const callback = useRef(onText)
  callback.current = onText
  const release = () => {
    stream.current?.getTracks().forEach((track) => track.stop())
    void meter.current?.close().catch(() => {})
    meter.current = null
    setAnalyser(null)
  }
  useEffect(
    () => () => {
      clearTimeout(timer.current)
      if (recorder.current?.state === 'recording') {
        recorder.current.onstop = null
        recorder.current.stop()
      }
      release()
    },
    []
  )
  function stopRecording(cancel = false) {
    return new Promise<string>((resolve) => {
      if (recorder.current?.state !== 'recording') return resolve('')
      discard.current = cancel
      pending.current = resolve
      recorder.current.stop()
    })
  }
  async function start() {
    try {
      const plugins = await workspace.call('plugins')
      const list = Array.isArray(plugins) ? plugins : plugins?.plugins || []
      const voice = list.find((plugin: any) => plugin.name === 'voice')
      if (!voice?.enabled) throw new Error(t('Включите плагин Voice в разделе «Плагины»'))
      const media = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, sampleRate: 16000 }
      })
      stream.current = media
      const level = new AudioContext()
      const node = level.createAnalyser()
      node.fftSize = 256
      level.createMediaStreamSource(media).connect(node)
      meter.current = level
      setAnalyser(node)
      const instance = new MediaRecorder(media)
      recorder.current = instance
      const chunks: Blob[] = []
      discard.current = false
      instance.ondataavailable = (event) => {
        if (event.data.size) chunks.push(event.data)
      }
      instance.onstop = async () => {
        clearTimeout(timer.current)
        setRecording(false)
        release()
        const resolve = pending.current
        pending.current = null
        if (discard.current) {
          resolve?.('')
          return
        }
        setTranscribing(true)
        let context: AudioContext | undefined
        let text = ''
        try {
          context = new AudioContext({ sampleRate: 16000 })
          const decoded = await context.decodeAudioData(
            await new Blob(chunks, { type: instance.mimeType }).arrayBuffer()
          )
          const offline = new OfflineAudioContext(1, Math.ceil(decoded.duration * 16000), 16000)
          const source = offline.createBufferSource()
          source.buffer = decoded
          source.connect(offline.destination)
          source.start()
          const rendered = await offline.startRendering()
          const result = await workspace.call('voice', {
            audio_base64: wav(rendered.getChannelData(0)),
            mime_type: 'audio/wav'
          })
          text = result?.text || ''
        } catch (error) {
          workspace.errorHandler(error)
        } finally {
          await context?.close()
          setTranscribing(false)
        }
        if (resolve) resolve(text)
        else if (text) callback.current(text)
      }
      instance.start()
      setRecording(true)
      timer.current = setTimeout(() => instance.state === 'recording' && instance.stop(), 120000)
    } catch (error) {
      release()
      workspace.errorHandler(error)
    }
  }
  async function toggle() {
    if (!recording) return start()
    const text = await stopRecording()
    if (text) callback.current(text)
  }
  return {
    recording,
    transcribing,
    analyser,
    toggle,
    finish: () => stopRecording(),
    cancel: () => stopRecording(true)
  }
}
