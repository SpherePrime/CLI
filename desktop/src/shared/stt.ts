export type AsrLanguage = 'ru' | 'en' | 'uk' | 'auto'

export const DEFAULT_ASR_LANGUAGE: AsrLanguage = 'ru'

export const ASR_LANGUAGES: readonly AsrLanguage[] = ['ru', 'en', 'uk', 'auto']

export type AsrState = 'idle' | 'downloading' | 'loading' | 'ready' | 'error'

export type AsrStatus = {
  state: AsrState
  ready: boolean
  languages: string[]
  chunkMs: number
  progress?: number | null
  message?: string | null
}

export type LiveTranscript = {
  committed: string
  open: string
  tail: string
}

export type AsrRequest =
  | { action: 'status' }
  | { action: 'warmup' }
  | { action: 'start'; language?: string; startLanguage?: string }
  | { action: 'push'; sessionId: string; pcmBase64: string }
  | { action: 'finish'; sessionId: string }
  | { action: 'cancel'; sessionId?: string }
  | { action: 'transcribe'; pcmBase64: string; language?: string; startLanguage?: string }

export type AsrEvent =
  | { type: 'status'; status: AsrStatus }
  | { type: 'partial'; sessionId: string; committed: string; open: string; tail: string }
  | { type: 'commit'; sessionId: string; text: string }
  | { type: 'eou'; sessionId: string; atSeconds: number; probability: number }
  | { type: 'language'; sessionId: string; language: string }
  | { type: 'final'; sessionId: string; text: string }
  | { type: 'error'; sessionId?: string; message: string }

export type SttStatus = {
  engine: 'tinyasr'
  model: string
  language: AsrLanguage
  languages: AsrLanguage[]
  installed: boolean
}

export function normalizeAsrLanguage(value?: string): AsrLanguage {
  const cleaned = String(value || '')
    .trim()
    .toLowerCase()
    .split(/[-_,]/)[0]
  if (!cleaned) return DEFAULT_ASR_LANGUAGE
  if (cleaned === 'auto') return 'auto'
  if (cleaned === 'ru' || cleaned === 'en' || cleaned === 'uk') return cleaned
  if (cleaned === 'ua') return 'uk'
  return DEFAULT_ASR_LANGUAGE
}
