export type AppUiBounds = {
  x: number
  y: number
  width: number
  height: number
}

export type AppUiProvider = 'uia' | 'atspi' | 'ax' | 'dom' | 'vision'
export type AppUiConfidence = 'high' | 'medium' | 'low'
export type AppUiAccessMode = 'view' | 'control'
export type AppUiElementAction = 'click' | 'toggle' | 'select' | 'focus'

export type AppUiNode = {
  ref: string
  parent?: string
  role: string
  name: string
  value?: string
  bounds: AppUiBounds
  states: string[]
  actions: string[]
  automationId?: string
  className?: string
  path: string
  source: AppUiProvider
  confidence: AppUiConfidence
  children: AppUiNode[]
}

export type AppUiSnapshot = {
  provider: AppUiProvider
  hwnd: string
  title: string
  capturedAt: number
  truncated: boolean
  warnings: string[]
  nodes: AppUiNode[]
  count: number
}

export type AppUiTarget = {
  hwnd: string
  title: string
  pid: number
  identity?: string
  bounds: AppUiBounds
}

export type AppUiLocator = {
  reference?: string
  automationId?: string
  role?: string
  name?: string
  path?: string
}

export type AppUiRequest =
  | { action: 'targets' }
  | {
      action: 'attach'
      hwnd: string
      pid: number
      identity: string
      title: string
      mode: AppUiAccessMode
    }
  | { action: 'detach' }
  | { action: 'snapshot' }
  | { action: 'act'; locator: AppUiLocator; elementAction: AppUiElementAction }
  | { action: 'type'; locator: AppUiLocator; text: string }
  | { action: 'state' }

export type AppUiLogEntry = {
  time: number
  action: string
  ok: boolean
  error?: string
}

export type AppUiShare = {
  mode: AppUiAccessMode
}

export type AppUiResponse = {
  snapshot?: AppUiSnapshot
  targets?: AppUiTarget[]
  share?: AppUiShare | null
  log?: AppUiLogEntry[]
}
