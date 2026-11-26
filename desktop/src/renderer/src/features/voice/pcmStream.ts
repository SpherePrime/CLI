const TARGET_SAMPLE_RATE = 16_000
const CHUNK_SAMPLES = 1536

const WORKLET_CODE = `
class PrimePcmCaptureProcessor extends AudioWorkletProcessor {
  process(inputs) {
    const channel = inputs[0] && inputs[0][0]
    if (channel && channel.length) {
      const copy = new Float32Array(channel.length)
      copy.set(channel)
      this.port.postMessage(copy, [copy.buffer])
    }
    return true
  }
}
registerProcessor('prime-pcm-capture', PrimePcmCaptureProcessor)
`

export function downsampleTo16k(
  samples: Float32Array,
  inputRate: number
): Float32Array {
  if (!samples.length) return new Float32Array(0)
  if (inputRate === TARGET_SAMPLE_RATE) return samples
  const ratio = inputRate / TARGET_SAMPLE_RATE
  const length = Math.max(1, Math.round(samples.length / ratio))
  const out = new Float32Array(length)
  for (let i = 0; i < length; i++) {
    const start = Math.floor(i * ratio)
    const end = Math.min(samples.length, Math.max(start + 1, Math.floor((i + 1) * ratio)))
    let sum = 0
    for (let j = start; j < end; j++) sum += samples[j]
    out[i] = sum / (end - start)
  }
  return out
}

export function float32ToS16LE(samples: Float32Array): Uint8Array {
  const buffer = new ArrayBuffer(samples.length * 2)
  const view = new DataView(buffer)
  for (let i = 0; i < samples.length; i++) {
    const clamped = Math.max(-1, Math.min(1, samples[i]))
    const int16 = clamped < 0 ? Math.round(clamped * 0x8000) : Math.round(clamped * 0x7fff)
    view.setInt16(i * 2, int16, true)
  }
  return new Uint8Array(buffer)
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  const step = 0x8000
  for (let offset = 0; offset < bytes.length; offset += step) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + step))
  }
  return btoa(binary)
}

export type PcmStreamHandle = {
  analyser: AnalyserNode
  close: () => Promise<void>
}

export type PcmStreamOptions = {
  deviceId?: string
  onChunk: (pcmS16le: Uint8Array, rms: number) => void
  onError?: (error: Error) => void
}

export async function openPcmStream(options: PcmStreamOptions): Promise<PcmStreamHandle> {
  const audioConstraint: MediaTrackConstraints = {
    channelCount: 1,
    echoCancellation: true,
    noiseSuppression: true,
    autoGainControl: true,
    ...(options.deviceId ? { deviceId: { exact: options.deviceId } } : {})
  }
  const stream = await navigator.mediaDevices.getUserMedia({ audio: audioConstraint })
  const context = new AudioContext()
  if (context.state === 'suspended') {
    await context.resume().catch(() => {})
  }

  const source = context.createMediaStreamSource(stream)
  const analyser = context.createAnalyser()
  analyser.fftSize = 1024
  analyser.smoothingTimeConstant = 0.72
  source.connect(analyser)

  const silentGain = context.createGain()
  silentGain.gain.value = 0
  silentGain.connect(context.destination)

  let pending = new Float32Array(0)
  let closed = false

  const emitSamples = (input: Float32Array) => {
    if (closed || !input.length) return
    const downsampled = downsampleTo16k(input, context.sampleRate)
    const merged = new Float32Array(pending.length + downsampled.length)
    merged.set(pending)
    merged.set(downsampled, pending.length)

    let offset = 0
    while (offset + CHUNK_SAMPLES <= merged.length) {
      const slice = merged.subarray(offset, offset + CHUNK_SAMPLES)
      let sumSq = 0
      for (let i = 0; i < slice.length; i++) sumSq += slice[i] * slice[i]
      const rms = Math.sqrt(sumSq / slice.length)
      options.onChunk(float32ToS16LE(slice), rms)
      offset += CHUNK_SAMPLES
    }
    pending = merged.slice(offset)
  }

  let cleanupNode: (() => void) | null = null

  if (context.audioWorklet && typeof Blob !== 'undefined' && typeof URL.createObjectURL === 'function') {
    const blob = new Blob([WORKLET_CODE], { type: 'application/javascript' })
    const moduleUrl = URL.createObjectURL(blob)
    try {
      await context.audioWorklet.addModule(moduleUrl)
      const worklet = new AudioWorkletNode(context, 'prime-pcm-capture')
      worklet.port.onmessage = (event) => {
        if (event.data instanceof Float32Array) emitSamples(event.data)
      }
      source.connect(worklet)
      worklet.connect(silentGain)
      cleanupNode = () => {
        worklet.port.onmessage = null
        worklet.disconnect()
      }
    } catch {
      cleanupNode = null
    } finally {
      URL.revokeObjectURL(moduleUrl)
    }
  }

  if (!cleanupNode) {
    const processor = context.createScriptProcessor(2048, 1, 1)
    processor.onaudioprocess = (event) => {
      emitSamples(event.inputBuffer.getChannelData(0))
    }
    source.connect(processor)
    processor.connect(silentGain)
    cleanupNode = () => {
      processor.onaudioprocess = null
      processor.disconnect()
    }
  }

  const flushRemaining = () => {
    if (pending.length > 0) {
      let sumSq = 0
      for (let i = 0; i < pending.length; i++) sumSq += pending[i] * pending[i]
      options.onChunk(float32ToS16LE(pending), Math.sqrt(sumSq / pending.length))
      pending = new Float32Array(0)
    }
  }

  return {
    analyser,
    async close() {
      if (closed) return
      flushRemaining()
      closed = true
      cleanupNode?.()
      source.disconnect()
      analyser.disconnect()
      silentGain.disconnect()
      stream.getTracks().forEach((track) => track.stop())
      await context.close().catch(() => {})
    }
  }
}
