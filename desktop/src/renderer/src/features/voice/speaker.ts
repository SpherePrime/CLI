import type { SpeechAudio } from '../../../../shared/speech'

type Options = {
  voice: string
  speed: number
  onState(speaking: boolean): void
  onError(error: unknown): void
}

export class Speaker {
  private context = new AudioContext()
  readonly analyser = this.context.createAnalyser()
  private source?: AudioBufferSourceNode
  private generation = 0
  private active = false

  constructor(private options: Options) {
    this.analyser.fftSize = 256
    this.analyser.smoothingTimeConstant = 0.72
    this.analyser.connect(this.context.destination)
  }

  update(options: Partial<Pick<Options, 'voice' | 'speed'>>) {
    Object.assign(this.options, options)
  }

  get speaking() {
    return this.active
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
