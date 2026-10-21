import type { SpeechAudio } from '../../../../shared/speech'

type Options = {
  voice: string
  speed: number
  // deviceId names the output device from the voice settings. Empty means
  // the system default.
  deviceId?: string
  onState(speaking: boolean): void
  onError(error: unknown): void
}

export class Speaker {
  private context!: AudioContext
  readonly analyser!: AnalyserNode
  private source?: AudioBufferSourceNode
  private generation = 0
  private active = false

  constructor(private options: Options) {
    // Field initializers would run before options are assigned, so the
    // context is created here with the configured output device.
    this.context = new AudioContext(sinkOptions(options.deviceId))
    this.analyser = this.context.createAnalyser()
    this.analyser.fftSize = 256
    this.analyser.smoothingTimeConstant = 0.72
    this.analyser.connect(this.context.destination)
  }

  update(options: Partial<Pick<Options, 'voice' | 'speed' | 'deviceId'>>) {
    Object.assign(this.options, options)
    void reroute(this.context, this.options.deviceId)
  }

  get speaking() {
    return this.active
  }

  // The output device this instance was created for, so callers can rebuild
  // it when the settings change.
  get deviceId() {
    return this.options.deviceId
  }

  private synthesize(text: string) {
    return window.prime.speech({
      action: 'speak',
      text,
      voice: this.options.voice,
      speed: this.options.speed
    }) as Promise<SpeechAudio>
  }

  private async decode(audio: SpeechAudio) {
    const bytes = Uint8Array.from(atob(audio.audio), (char) => char.charCodeAt(0))
    return this.context.decodeAudioData(bytes.buffer)
  }

  private play(buffer: AudioBuffer, generation: number) {
    return new Promise<void>((resolve) => {
      if (generation !== this.generation) return resolve()
      const source = this.context.createBufferSource()
      source.buffer = buffer
      source.connect(this.analyser)
      source.onended = () => resolve()
      this.source = source
      source.start()
    })
  }

  async speak(chunks: string[]) {
    const generation = ++this.generation
    if (!chunks.length) return
    if (this.context.state === 'suspended') await this.context.resume()
    this.active = true
    this.options.onState(true)
    try {
      let next = this.synthesize(chunks[0])
      for (let index = 0; index < chunks.length; index++) {
        const audio = await next
        if (generation !== this.generation) return
        if (index + 1 < chunks.length) next = this.synthesize(chunks[index + 1])
        await this.play(await this.decode(audio), generation)
        if (generation !== this.generation) return
      }
    } catch (error) {
      if (generation === this.generation) this.options.onError(error)
    } finally {
      if (generation === this.generation) {
        this.active = false
        this.source = undefined
        this.options.onState(false)
      }
    }
  }

  stop() {
    this.generation++
    try {
      this.source?.stop()
    } catch {}
    this.source = undefined
    if (this.active) {
      this.active = false
      this.options.onState(false)
    }
  }

  async dispose() {
    this.stop()
    this.analyser.disconnect()
    await this.context.close().catch(() => {})
  }
}

// sinkOptions picks the configured output device at context creation.
// setSinkId is typed loosely because the DOM lib in this repo does not ship
// the audio-output types yet, though the runtime (Chromium 130+) has them.
function sinkOptions(deviceId: string | undefined): AudioContextOptions | undefined {
  if (!deviceId) return undefined
  return { sinkId: deviceId } as AudioContextOptions
}

async function reroute(context: AudioContext, deviceId: string | undefined) {
  try {
    if (deviceId)
      await (context as unknown as { setSinkId(id: string): Promise<void> }).setSinkId(deviceId)
  } catch {
    // A missing or busy device keeps the previous output; the next speech
    // turn retries through a fresh context.
  }
}
