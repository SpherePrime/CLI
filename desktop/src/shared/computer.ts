export type WindowInfo = {
  hwnd: string
  title: string
  pid: number
  bounds: { x: number; y: number; width: number; height: number }
}

export type WindowShare = { workspaceId: string; sessionId: string; hwnd: string; title: string; sharedAt: number }

export type WindowLogEntry = { time: number; title: string; source: 'user' | 'agent'; ok: boolean; error?: string }

export type WindowState = { share?: WindowShare; log: WindowLogEntry[] }

export type WindowRequest =
  | { action: 'list' | 'state' | 'revoke' }
  | { action: 'snapshot'; hwnd: string }
  | { action: 'share'; hwnd: string; sessionId: string }
