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
      ? 48
      : Math.max(210, Math.min(400, sidebarWidth))
    : 0
  const inspectorMax = Math.max(280, width - (sidebarVisible ? 48 : 0) - 122)
  const inspector = inspectorVisible
    ? compact
      ? 48
      : Math.max(280, Math.min(inspectorMax, inspectorWidth))
    : 0
  const columns = [
    sidebar ? `${sidebar}px` : '',
    sidebar && !sidebarCompact ? '1px' : '',
    'minmax(0, 1fr)',
    inspector && !compact ? '1px' : '',
    inspector ? `${inspector}px` : ''
  ]
    .filter(Boolean)
    .join(' ')
  return { compact, sidebarCompact, sidebar, inspector, inspectorMax, columns }
}
