export type AppUiBounds = {
  x: number
  y: number
  width: number
  height: number
}

export type AppUiProvider = 'uia' | 'atspi' | 'ax' | 'dom' | 'vision'
export type AppUiConfidence = 'high' | 'medium' | 'low'
export type AppUiAccessMode = 'view' | 'control'

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
  | { action: 'clearReport' }
  | { action: 'state' }

export type AppUiLogEntry = {
  time: number
  title: string
  source: string
  action: string
  ok: boolean
  error?: string
}

export type AppUiCheck = {
  time: number
  name: string
  passed: boolean
  details?: string
}

export type AppUiIssueSeverity = 'error' | 'warning' | 'info'

export type AppUiIssue = {
  kind: string
  severity: AppUiIssueSeverity
  refs: string[]
  message: string
}

export type AppUiAudit = {
  checked: number
  errors: number
  warnings: number
  issues: AppUiIssue[]
}

export type AppUiReport = {
  checks: AppUiCheck[]
  audit?: AppUiAudit
}

export type AppUiLaunched = {
  command: string
  pid: number
  running: boolean
}

export type AppUiState = {
  target?: AppUiTarget | null
  log?: AppUiLogEntry[]
  launched?: AppUiLaunched | null
  report?: AppUiReport
}

export type AppUiResponse = AppUiState & {
  snapshot?: AppUiSnapshot
  windows?: AppUiTarget[]
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
