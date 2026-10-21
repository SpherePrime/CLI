import type React from 'react'
import type { Workspace } from './useWorkspace'
import { railWidth } from './panelGeometry'

type Panel = 'sidebar' | 'inspector'

const limits = { sidebar: { min: 210, max: 400 }, inspector: { min: 280 } }
const collapseDistance = 60

export function usePanelResize({
  workspace: w,
  widths,
  setWidths,
  collapsed,
  setCollapsed,
  inspectorMax,
  zoom,
  beginInspectorResize
}: {
  workspace: Workspace
  widths: Record<Panel, number>
  setWidths: React.Dispatch<React.SetStateAction<Record<Panel, number>>>
  collapsed: Record<Panel, boolean>
  setCollapsed: (which: Panel, value: boolean, persist?: boolean) => void
  inspectorMax: number
  zoom: number
  beginInspectorResize?: () => (width: number) => void
}) {
  const maximum = (which: Panel) => (which === 'sidebar' ? limits.sidebar.max : inspectorMax)
  const clamp = (which: Panel, width: number) =>
    Math.max(limits[which].min, Math.min(maximum(which), width))
  function persistWidth(which: Panel, width: number) {
    void w
      .savePreferences({ [which === 'sidebar' ? 'sidebarWidth' : 'inspectorWidth']: width })
      .catch(w.errorHandler)
  }
  function apply(
    which: Panel,
    raw: number,
    persist = false,
    resizeEditor?: (width: number) => void
  ) {
    const collapse = raw < limits[which].min - collapseDistance
    setCollapsed(which, collapse, persist)
    if (collapse) {
      resizeEditor?.(0)
      return
    }
    const next = clamp(which, raw)
    resizeEditor?.(next)
    setWidths((value) => ({ ...value, [which]: next }))
    if (persist) persistWidth(which, next)
  }
  function pointer(which: Panel, event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    const start = event.clientX
    const initial = collapsed[which] ? limits[which].min - collapseDistance : widths[which]
    let latest = initial
    const resizeEditor = which === 'inspector' ? beginInspectorResize?.() : undefined
    const move = (move: PointerEvent) => {
      latest = initial + ((move.clientX - start) / zoom) * (which === 'sidebar' ? 1 : -1)
      apply(which, latest, false, resizeEditor)
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      window.removeEventListener('pointercancel', stop)
      if (latest !== initial) apply(which, latest, true, resizeEditor)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop)
    window.addEventListener('pointercancel', stop)
  }
  function keyboard(which: Panel, event: React.KeyboardEvent<HTMLDivElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const resizeEditor = which === 'inspector' ? beginInspectorResize?.() : undefined
    if (event.key === 'Home') return apply(which, 0, true, resizeEditor)
    if (event.key === 'End') return apply(which, maximum(which), true, resizeEditor)
    const direction = (event.key === 'ArrowRight' ? 1 : -1) * (which === 'sidebar' ? 1 : -1)
    if (collapsed[which]) {
      if (direction > 0) apply(which, limits[which].min, true, resizeEditor)
      return
    }
    const next = widths[which] + direction * (event.shiftKey ? 50 : 10)
    apply(which, next < limits[which].min ? 0 : next, true, resizeEditor)
  }
  const separator = (which: Panel, label: string, value: number) => ({
    className: `resizer ${which === 'sidebar' ? 'left-resizer' : 'right-resizer'}`,
    role: 'separator',
    'aria-label': label,
    'aria-orientation': 'vertical' as const,
    'aria-valuemin': railWidth,
    'aria-valuemax': maximum(which),
    'aria-valuenow': value,
    tabIndex: 0,
    onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => pointer(which, event),
    onKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => keyboard(which, event)
  })
  return { separator }
}
