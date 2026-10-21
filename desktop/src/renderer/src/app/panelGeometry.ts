export const panelGutter = 8
export const railWidth = 48
const minimumChat = 360

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
  const rails = (sidebarVisible ? 1 : 0) + (inspectorVisible ? 1 : 0)
  const sidebarResizable = sidebarVisible && !compact
  const inspectorResizable = inspectorVisible && !compact
  const inspectorMax = Math.max(
    280,
    width - rails * (railWidth + panelGutter) - panelGutter - 122
  )
  const inspectorOpen = inspectorResizable && !inspectorCollapsed
  const inspector = inspectorOpen ? Math.max(280, Math.min(inspectorMax, inspectorWidth)) : 0
  const sidebarCandidate = Math.max(210, Math.min(400, sidebarWidth))
  const sidebarFits =
    width -
      rails * (railWidth + panelGutter) -
      sidebarCandidate -
      panelGutter -
      (inspector ? inspector + panelGutter : 0) >=
    minimumChat
  const sidebarOpen = sidebarResizable && !sidebarCollapsed && sidebarFits
  const sidebar = sidebarOpen ? sidebarCandidate : 0
  const rail = (open: boolean, resizable: boolean) =>
    `${resizable && !open ? railWidth : railWidth + panelGutter}px`
  const columns = [
    sidebarVisible ? rail(sidebarOpen, sidebarResizable) : '',
    sidebarOpen ? `${sidebar}px` : '',
    sidebarResizable ? `${panelGutter}px` : '',
    'minmax(0, 1fr)',
    inspectorResizable ? `${panelGutter}px` : '',
    inspectorOpen ? `${inspector}px` : '',
    inspectorVisible ? rail(inspectorOpen, inspectorResizable) : ''
  ]
    .filter(Boolean)
    .join(' ')
  return {
    compact,
    sidebarOpen,
    inspectorOpen,
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
  const side = (visible: boolean, panel: number) =>
    visible ? railWidth + panelGutter + (panel ? panel + panelGutter : 0) : 0
  return Math.round(
    (side(inspectorVisible, layout.inspector) - side(sidebarVisible, layout.sidebar)) / 2
  )
}
