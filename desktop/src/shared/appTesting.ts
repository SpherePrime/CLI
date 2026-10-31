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
      sessionId: string
      hwnd: string
      pid: number
      identity: string
      title: string
      mode: AppUiAccessMode
    }
  | { action: 'detach' }
  | { action: 'snapshot'; sessionId: string }
  | { action: 'act'; sessionId: string; locator: AppUiLocator; elementAction: AppUiElementAction }
  | { action: 'type'; sessionId: string; locator: AppUiLocator; text: string }
  | { action: 'testList' }
  | { action: 'testRun'; sessionId: string; path: string }
  | { action: 'explore'; sessionId: string; maxSteps?: number }
  | { action: 'recordStart' }
  | { action: 'recordStop'; sessionId: string }
  | { action: 'state' }

export type AppUiLogEntry = {
  time: number
  title: string
  source: string
  action: string
  ok: boolean
  error?: string
}

export type AppUiResponse = {
  text?: string
  snapshot?: AppUiSnapshot
  windows?: AppUiTarget[]
  target?: AppUiTarget | null
  log?: AppUiLogEntry[]
  scenarios?: AppUiScenario[]
  run?: AppUiRun
  explore?: AppUiExplore
  record?: AppUiRecord
}

export type AppUiExplore = {
  visited: number
  blocked: { ref: string; role: string; name: string }[]
  steps: { index: number; role: string; name: string; ok: boolean; error?: string | null }[]
  stopped: string
}

export type AppUiRecord = {
  path: string
  draft: string
  steps: number
}

export type AppUiScenario = {
  path: string
  name?: string
  steps?: number
  launch?: string | null
  error?: string
}

export type AppUiRunStep = {
  index: number
  action: string
  ok: boolean
  ms: number
  error?: string | null
}

export type AppUiRun = {
  scenario: string
  name?: string | null
  passed: boolean
  steps: AppUiRunStep[]
  artifacts: string
}

export type AppUiPickedElement = {
  reference?: string | null
  role: string
  name: string
  value?: string | null
  bounds: AppUiBounds
  states: string[]
  actions: string[]
  automationId?: string | null
  className?: string | null
  path: string
  source: AppUiProvider
  confidence: AppUiConfidence
}

export type PickedAppElement = {
  element: AppUiPickedElement
  reference?: string | null
  confidence: AppUiConfidence
  candidates: number
  window: string
  attachmentId?: string
  image?: string
}
