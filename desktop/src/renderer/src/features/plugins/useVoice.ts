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
  const recorder = useRef<MediaRecorder | null>(null)
  const stream = useRef<MediaStream | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const callback = useRef(onText)
  callback.current = onText
  useEffect(
    () => () => {
      clearTimeout(timer.current)
      if (recorder.current?.state === 'recording') {
        recorder.current.onstop = null
        recorder.current.stop()
      }
      stream.current?.getTracks().forEach((track) => track.stop())
    },
    []
  )
  async function toggle() {
    if (recording) {
      recorder.current?.stop()
      return
    }
    try {
      const plugins = await workspace.call('plugins')
      const list = Array.isArray(plugins) ? plugins : plugins?.plugins || []
      const voice = list.find((plugin: any) => plugin.name === 'voice')
      if (!voice?.enabled) throw new Error(t('Включите плагин Voice в разделе «Плагины»'))
      const media = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, sampleRate: 16000 }
      })
      stream.current = media
      const instance = new MediaRecorder(media)
      recorder.current = instance
      const chunks: Blob[] = []
      instance.ondataavailable = (event) => {
        if (event.data.size) chunks.push(event.data)
      }
      instance.onstop = async () => {
        clearTimeout(timer.current)
        setRecording(false)
        media.getTracks().forEach((track) => track.stop())
        setTranscribing(true)
        let context: AudioContext | undefined
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
          if (result.text) callback.current(result.text)
        } catch (error) {
          workspace.errorHandler(error)
        } finally {
          await context?.close()
          setTranscribing(false)
        }
      }
      instance.start()
      setRecording(true)
      timer.current = setTimeout(() => instance.state === 'recording' && instance.stop(), 120000)
    } catch (error) {
      workspace.errorHandler(error)
    }
  }
  return { recording, transcribing, toggle }
}
