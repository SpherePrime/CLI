export const panelGutter = 8
const railWidth = 48

export function panelGeometry(
  width: number,
  sidebarWidth: number,
  inspectorWidth: number,
  sidebarVisible: boolean,
  inspectorVisible: boolean
) {
  const compact = width < 1050
  const sidebarCompact =
    compact || (inspectorVisible && width - sidebarWidth - inspectorWidth < 360)
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
    ? compact
      ? railWidth
      : Math.max(280, Math.min(inspectorMax, inspectorWidth))
    : 0
  const columns = [
    sidebar ? `${sidebarCompact ? sidebar + panelGutter : sidebar}px` : '',
    sidebar && !sidebarCompact ? `${panelGutter}px` : '',
    'minmax(0, 1fr)',
    inspector && !compact ? `${panelGutter}px` : '',
    inspector ? `${compact ? inspector + panelGutter : inspector}px` : ''
  ]
    .filter(Boolean)
    .join(' ')
  return { compact, sidebarCompact, sidebar, inspector, inspectorMax, columns }
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
