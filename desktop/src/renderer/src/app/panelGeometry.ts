export const panelGutter = 8
export const railWidth = 48

export function panelGeometry(
  width: number,
  sidebarWidth: number,
  inspectorWidth: number,
  sidebarVisible: boolean,
  inspectorVisible: boolean,
  sidebarCollapsed = false,
  inspectorCollapsed = false
) {
  const compact = width < 1050
  const inspectorCompact = compact || inspectorCollapsed
  const sidebarCrowded =
    inspectorVisible && !inspectorCompact && width - sidebarWidth - inspectorWidth < 360
  const sidebarCompact = compact || sidebarCollapsed || sidebarCrowded
  const sidebarResizable = sidebarVisible && !compact && (!sidebarCompact || sidebarCollapsed)
  const inspectorResizable = inspectorVisible && !compact
  const sidebar = sidebarVisible
    ? sidebarCompact
      ? railWidth
      : Math.max(210, Math.min(400, sidebarWidth))
    : 0
  const inspectorMax = Math.max(
    280,
    width - (sidebarVisible ? railWidth + panelGutter : 0) - panelGutter - 122
  )
  const inspector = inspectorVisible
    ? inspectorCompact
      ? railWidth
      : Math.max(280, Math.min(inspectorMax, inspectorWidth))
    : 0
  const track = (size: number, rail: boolean, resizable: boolean) =>
    `${rail && !resizable ? size + panelGutter : size}px`
  const columns = [
    sidebar ? track(sidebar, sidebarCompact, sidebarResizable) : '',
    sidebarResizable ? `${panelGutter}px` : '',
    'minmax(0, 1fr)',
    inspectorResizable ? `${panelGutter}px` : '',
    inspector ? track(inspector, inspectorCompact, inspectorResizable) : ''
  ]
    .filter(Boolean)
    .join(' ')
  return {
    compact,
    sidebarCompact,
    inspectorCompact,
    sidebarResizable,
    inspectorResizable,
    sidebar,
    inspector,
    inspectorMax,
    columns
  }
}

export function chatCenterShift(
  layout: ReturnType<typeof panelGeometry>,
  sidebarVisible: boolean,
  inspectorVisible: boolean,
  editorVisible: boolean
) {
  if (editorVisible) return 0
  const left = sidebarVisible ? layout.sidebar + panelGutter : 0
  const right = inspectorVisible ? layout.inspector + panelGutter : 0
  return Math.round((right - left) / 2)
}
