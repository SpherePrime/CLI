export type WindowInfo = {
  hwnd: string
  title: string
  pid: number
  identity?: string
  bounds: { x: number; y: number; width: number; height: number }
}

export type WindowAccessMode = 'view' | 'control'
export type WindowShare = {
  workspaceId: string
  sessionId: string
  hwnd: string
  title: string
  pid: number
  identity: string
  mode: WindowAccessMode
  sharedAt: number
}
export type WindowAccessRequest = {
  hwnd: string
  pid: number
  identity: string
  title: string
  sessionId: string
  mode: WindowAccessMode
  requestedAt: number
}
export type WindowLogEntry = {
  time: number
  title: string
  source: 'user' | 'agent'
  ok: boolean
  action?: string
  error?: string
}
export type WindowState = {
  share?: WindowShare
  request?: WindowAccessRequest
  log: WindowLogEntry[]
}
export type WindowAction =
  | { action: 'focus' }
  | { action: 'click'; x: number; y: number; button?: 'left' | 'right'; count?: number }
  | { action: 'type'; text: string }
  | { action: 'press_key'; key: string }
  | { action: 'scroll'; x: number; y: number; delta: number }

export type WindowRequest =
  | { action: 'list' | 'state' | 'revoke' | 'dismiss' }
  | { action: 'snapshot'; hwnd: string }
  | {
      action: 'share'
      hwnd: string
      sessionId: string
      mode?: WindowAccessMode
      expectedIdentity?: string
      expectedPid?: number
    }
