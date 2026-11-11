import { isForViewedChat } from './useBrowserView'

export function isInspectorOpen(
  compact: boolean,
  compactPanelOpen: boolean,
  showInspector: boolean,
  inspectorCollapsed: boolean
): boolean {
  return compact ? compactPanelOpen : showInspector && !inspectorCollapsed
}

export function shouldActivateBrowserTab(
  payload: { action?: string; sessionId?: unknown } | undefined,
  viewedSessionId: string | null,
  inspectorOpen: boolean
): boolean {
  return Boolean(
    payload?.action === 'activate' &&
      inspectorOpen &&
      isForViewedChat(payload?.sessionId, viewedSessionId)
  )
}
