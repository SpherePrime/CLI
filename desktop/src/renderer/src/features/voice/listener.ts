type Options = {
  silenceMs: number
  onSpeechStart(): void
  onSegment(samples: Float32Array, sampleRate: number): void
  sensitivity(): number
}

const frameSize = 2048
const prerollMs = 400
const minimumSpeechMs = 160
const minimumSegmentMs = 450
const maximumSegmentMs = 30000

export class Listener {
  private stream?: MediaStream
  private context?: AudioContext
  private processor?: ScriptProcessorNode
  private source?: MediaStreamAudioSourceNode
  analyser?: AnalyserNode
  private preroll: Float32Array[] = []
  private segment: Float32Array[] = []
  private speaking = false
  private voicedMs = 0
  private silentMs = 0
  private segmentMs = 0
  private floor = 0.006
  private muted = false

  constructor(private options: Options) {}

  async start() {
    this.stream = await navigator.mediaDevices.getUserMedia({
      audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true }
    })
    this.context = new AudioContext()
    this.source = this.context.createMediaStreamSource(this.stream)
    this.analyser = this.context.createAnalyser()
    this.analyser.fftSize = 256
    this.processor = this.context.createScriptProcessor(frameSize, 1, 1)
    this.source.connect(this.analyser)
    this.source.connect(this.processor)
    this.processor.connect(this.context.destination)
    this.processor.onaudioprocess = (event) => this.frame(event.inputBuffer.getChannelData(0))
  }

  setMuted(muted: boolean) {
    this.muted = muted
    this.stream?.getAudioTracks().forEach((track) => (track.enabled = !muted))
    if (muted) this.reset()
  }

  private reset() {
    this.speaking = false
    this.segment = []
    this.voicedMs = 0
    this.silentMs = 0
    this.segmentMs = 0
  }

  private frame(input: Float32Array) {
    if (this.muted || !this.context) return
    const samples = new Float32Array(input)
    const duration = (samples.length / this.context.sampleRate) * 1000
    let energy = 0
    for (const value of samples) energy += value * value
    const level = Math.sqrt(energy / samples.length)
    const threshold = Math.max(0.012, this.floor * 3) * this.options.sensitivity()
    const voiced = level > threshold
    if (!this.speaking) {
      this.floor = voiced ? this.floor : this.floor * 0.95 + level * 0.05
      this.preroll.push(samples)
      while (this.preroll.length * duration > prerollMs) this.preroll.shift()
      this.voicedMs = voiced ? this.voicedMs + duration : 0
      if (this.voicedMs >= minimumSpeechMs) {
        this.speaking = true
        this.segment = [...this.preroll]
        this.segmentMs = this.segment.length * duration
        this.silentMs = 0
        this.preroll = []
        this.options.onSpeechStart()
      }
      return
    }
    this.segment.push(samples)
    this.segmentMs += duration
    this.silentMs = voiced ? 0 : this.silentMs + duration
    if (this.silentMs >= this.options.silenceMs || this.segmentMs >= maximumSegmentMs) {
      const chunks = this.segment
      const spoken = this.segmentMs - this.silentMs
      this.reset()
      if (spoken < minimumSegmentMs) return
      const merged = new Float32Array(chunks.reduce((sum, chunk) => sum + chunk.length, 0))
      let offset = 0
      for (const chunk of chunks) {
        merged.set(chunk, offset)
        offset += chunk.length
      }
      this.options.onSegment(merged, this.context.sampleRate)
    }
  }

  async stop() {
    this.processor?.disconnect()
    this.source?.disconnect()
    this.analyser?.disconnect()
    if (this.processor) this.processor.onaudioprocess = null
    this.stream?.getTracks().forEach((track) => track.stop())
    await this.context?.close().catch(() => {})
    this.stream = undefined
    this.context = undefined
    this.analyser = undefined
    this.reset()
  }
}
