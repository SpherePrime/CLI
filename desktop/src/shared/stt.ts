export type SttModel = {
  id: string
  label: string
  language: string
  size_bytes: number
  recommended: boolean
  installed: boolean
  active: boolean
}

export type SttProvider = {
  id: 'google' | 'whisper' | 'vosk'
  title: string
  description: string
  available: boolean
  models: SttModel[]
}

export type SttStatus = {
  engine: string
  model: string
  providers: SttProvider[]
}

export type SttTask = {
  name: string
  state: 'running' | 'completed' | 'failed'
  phase: string
  text: string
  done: number
  total: number
  error: string
}
