import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { availableParallelism } from 'node:os'
import type { InferenceSession, Tensor } from 'onnxruntime-node'

type Ort = typeof import('onnxruntime-node')
type Style = { ttl: Tensor; dp: Tensor }
type Config = { ae: { sample_rate: number; base_chunk_size: number }; ttl: { chunk_compress_factor: number; latent_dim: number } }

const languages = new Set(['en', 'ko', 'ja', 'ar', 'bg', 'cs', 'da', 'de', 'el', 'es', 'et', 'fi', 'fr', 'hi', 'hr', 'hu', 'id', 'it', 'lt', 'lv', 'nl', 'pl', 'pt', 'ro', 'ru', 'sk', 'sl', 'sv', 'tr', 'uk', 'vi', 'na'])

const replacements: [string, string][] = [
  ['–', '-'], ['‑', '-'], ['—', '-'], ['_', ' '], ['“', '"'], ['”', '"'], ['‘', "'"], ['’', "'"],
  ['´', "'"], ['`', "'"], ['[', ' '], [']', ' '], ['|', ' '], ['/', ' '], ['#', ' '], ['→', ' '], ['←', ' '], ['@', ' at ']
]

function prepare(text: string, language: string) {
  let value = text.normalize('NFKD').replace(/\p{Extended_Pictographic}/gu, '')
  for (const [from, to] of replacements) value = value.replaceAll(from, to)
  value = value
    .replace(/[♥☆♡©\\]/g, '')
    .replace(/ ([,.!?;:'])/g, '$1')
    .replace(/"{2,}/g, '"')
    .replace(/'{2,}/g, "'")
    .replace(/\s+/g, ' ')
    .trim()
  if (!/[.!?;:,'"')\]}…»]$/.test(value)) value += '.'
  return `<${language}>${value}</${language}>`
}

export function detectLanguage(text: string) {
  const cyrillic = (text.match(/[а-яё]/gi) || []).length
  const latin = (text.match(/[a-z]/gi) || []).length
  if (/[іїєґ]/i.test(text)) return 'uk'
  return cyrillic >= latin ? 'ru' : 'en'
}

export class Supertonic {
  private constructor(
    private ort: Ort,
    private config: Config,
    private indexer: number[],
    private sessions: Record<'duration' | 'encoder' | 'estimator' | 'vocoder', InferenceSession>,
    private directory: string
  ) {}

  private styles = new Map<string, Style>()

  static async load(directory: string) {
    const ort = await import('onnxruntime-node')
    const onnx = join(directory, 'onnx')
    const config = JSON.parse(await readFile(join(onnx, 'tts.json'), 'utf8'))
    const indexer = JSON.parse(await readFile(join(onnx, 'unicode_indexer.json'), 'utf8'))
    const create = (name: string) =>
      ort.InferenceSession.create(join(onnx, name), {
        executionProviders: ['cpu'],
        graphOptimizationLevel: 'all',
        intraOpNumThreads: Math.max(1, Math.min(8, availableParallelism() - 1))
      })
    const [duration, encoder, estimator, vocoder] = await Promise.all([
      create('duration_predictor.onnx'),
      create('text_encoder.onnx'),
      create('vector_estimator.onnx'),
      create('vocoder.onnx')
    ])
    return new Supertonic(ort, config, indexer, { duration, encoder, estimator, vocoder }, directory)
  }

  get sampleRate() {
    return this.config.ae.sample_rate
  }

  private async style(voice: string): Promise<Style> {
    const cached = this.styles.get(voice)
    if (cached) return cached
    const file = voice.startsWith('custom:')
      ? join(this.directory, '..', 'voices', `${voice.slice(7)}.json`)
      : join(this.directory, 'voice_styles', `${voice}.json`)
    const value = JSON.parse(await readFile(file, 'utf8'))
    const tensor = (entry: { dims: number[]; data: unknown[] }) =>
      new this.ort.Tensor('float32', Float32Array.from(entry.data.flat(Infinity) as number[]), entry.dims)
    const style = { ttl: tensor(value.style_ttl), dp: tensor(value.style_dp) }
    this.styles.set(voice, style)
    return style
  }

  async synthesize(text: string, options: { voice: string; speed: number; language?: string; steps?: number }) {
    const language = options.language && languages.has(options.language) ? options.language : detectLanguage(text)
    const style = await this.style(options.voice)
    const prepared = prepare(text, language)
    const codes = Array.from(prepared, (char) => this.indexer[char.charCodeAt(0)] ?? 0)
    const length = codes.length
    const textIds = new this.ort.Tensor('int64', BigInt64Array.from(codes.map((code) => BigInt(code))), [1, length])
    const textMask = new this.ort.Tensor('float32', new Float32Array(length).fill(1), [1, 1, length])
    const durationResult = await this.sessions.duration.run({ text_ids: textIds, style_dp: style.dp, text_mask: textMask })
    const seconds = (durationResult.duration.data as Float32Array)[0] / options.speed
    const encoded = await this.sessions.encoder.run({ text_ids: textIds, style_ttl: style.ttl, text_mask: textMask })
    const { base_chunk_size } = this.config.ae
    const { chunk_compress_factor, latent_dim } = this.config.ttl
    const chunk = base_chunk_size * chunk_compress_factor
    const samples = Math.floor(seconds * this.sampleRate)
    const latentLength = Math.max(1, Math.floor((samples + chunk - 1) / chunk))
    const dimension = latent_dim * chunk_compress_factor
    let latent = new Float32Array(dimension * latentLength)
    for (let index = 0; index < latent.length; index++) {
      const u1 = Math.max(1e-10, Math.random())
      latent[index] = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * Math.random())
    }
    const latentMask = new this.ort.Tensor('float32', new Float32Array(latentLength).fill(1), [1, 1, latentLength])
    const steps = options.steps ?? 5
    const total = new this.ort.Tensor('float32', Float32Array.of(steps), [1])
    for (let step = 0; step < steps; step++) {
      const result = await this.sessions.estimator.run({
        noisy_latent: new this.ort.Tensor('float32', latent, [1, dimension, latentLength]),
        text_emb: encoded.text_emb,
        style_ttl: style.ttl,
        text_mask: textMask,
        latent_mask: latentMask,
        total_step: total,
        current_step: new this.ort.Tensor('float32', Float32Array.of(step), [1])
      })
      latent = Float32Array.from(result.denoised_latent.data as Float32Array)
    }
    const audio = await this.sessions.vocoder.run({ latent: new this.ort.Tensor('float32', latent, [1, dimension, latentLength]) })
    const wav = audio.wav_tts.data as Float32Array
    return wav.subarray(0, Math.min(wav.length, samples))
  }

  forget(voice: string) {
    this.styles.delete(voice)
  }

  async release() {
    await Promise.all(Object.values(this.sessions).map((session) => session.release().catch(() => {})))
  }
}

export function encodeWav(samples: Float32Array, sampleRate: number) {
  const buffer = Buffer.alloc(44 + samples.length * 2)
  buffer.write('RIFF', 0)
  buffer.writeUInt32LE(36 + samples.length * 2, 4)
  buffer.write('WAVE', 8)
  buffer.write('fmt ', 12)
  buffer.writeUInt32LE(16, 16)
  buffer.writeUInt16LE(1, 20)
  buffer.writeUInt16LE(1, 22)
  buffer.writeUInt32LE(sampleRate, 24)
  buffer.writeUInt32LE(sampleRate * 2, 28)
  buffer.writeUInt16LE(2, 32)
  buffer.writeUInt16LE(16, 34)
  buffer.write('data', 36)
  buffer.writeUInt32LE(samples.length * 2, 40)
  for (let index = 0; index < samples.length; index++)
    buffer.writeInt16LE(Math.round(Math.max(-1, Math.min(1, samples[index])) * 32767), 44 + index * 2)
  return buffer
}
