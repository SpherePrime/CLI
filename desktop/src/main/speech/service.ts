import { downloadSpeechModel, speechModelDirectory, speechModelReady } from './assets'
import { Supertonic, encodeWav } from './supertonic'
import { importVoice, listVoices, removeVoice, voiceExists } from './voices'
import type { SpeechRequest, SpeechStatus } from '../../shared/speech'

export class SpeechService {
  private engine?: Promise<Supertonic>
  private downloading?: Promise<string>
  private state: SpeechStatus = { state: 'unknown' }
  private queue: Promise<unknown> = Promise.resolve()

  constructor(
    private emit: (status: SpeechStatus) => void,
    private chooseFile: () => Promise<string | undefined>
  ) {}

  private set(status: SpeechStatus) {
    this.state = status
    this.emit(status)
  }

  async status(): Promise<SpeechStatus> {
    if (this.state.state === 'unknown')
      this.state = (await speechModelReady()) ? { state: 'ready' } : { state: 'missing' }
    return this.state
  }

  private load() {
    this.engine ??= (async () => {
      if (!(await speechModelReady())) {
        this.downloading ??= downloadSpeechModel((percent) => this.set({ state: 'downloading', percent }))
        try {
          await this.downloading
        } finally {
          this.downloading = undefined
        }
      }
      this.set({ state: 'loading' })
      const engine = await Supertonic.load(speechModelDirectory())
      this.set({ state: 'ready' })
      return engine
    })().catch((error) => {
      this.engine = undefined
      this.set({ state: 'error', error: (error as Error).message })
      throw error
    })
    return this.engine
  }

  async handle(request: SpeechRequest) {
    switch (request.action) {
      case 'status':
        return this.status()
      case 'prepare':
        await this.load()
        return this.state
      case 'voices':
        return listVoices()
      case 'import': {
        const path = await this.chooseFile()
        return path ? importVoice(path) : null
      }
      case 'remove': {
        await removeVoice(request.voice)
        ;(await this.engine?.catch(() => undefined))?.forget(request.voice)
        return listVoices()
      }
      case 'speak':
        return this.speak(request)
    }
  }

  private async speak(request: Extract<SpeechRequest, { action: 'speak' }>) {
    const text = String(request.text ?? '').slice(0, 4000).trim()
    if (!text) throw new Error('Пустой текст')
    const voice = request.voice && (await voiceExists(request.voice)) ? request.voice : 'F1'
    const speed = Math.min(1.6, Math.max(0.7, Number(request.speed) || 1.05))
    const operation = this.queue.then(async () => {
      const engine = await this.load()
      const samples = await engine.synthesize(text, { voice, speed })
      return { audio: encodeWav(samples, engine.sampleRate).toString('base64'), mime: 'audio/wav' }
    })
    this.queue = operation.catch(() => {})
    return operation
  }

  async dispose() {
    const engine = await this.engine?.catch(() => undefined)
    await engine?.release()
  }
}
