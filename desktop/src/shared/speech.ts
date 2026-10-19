export type SpeechState = 'unknown' | 'missing' | 'downloading' | 'loading' | 'ready' | 'error'

export type SpeechStatus = { state: SpeechState; percent?: number; error?: string }

export type SpeechVoice = { id: string; name: string; custom: boolean }

export type SpeechRequest =
  | { action: 'status' | 'prepare' | 'voices' | 'import' }
  | { action: 'remove'; voice: string }
  | { action: 'speak'; text: string; voice?: string; speed?: number }

export type SpeechAudio = { audio: string; mime: string }
