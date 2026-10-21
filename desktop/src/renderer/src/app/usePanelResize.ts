import type React from 'react'
import type { Workspace } from './useWorkspace'

type Panel = 'sidebar' | 'inspector'

const limits = { sidebar: { min: 210, max: 400 }, inspector: { min: 280 } }

export function usePanelResize({
  workspace: w,
  widths,
  setWidths,
  inspectorMax,
  zoom
}: {
  workspace: Workspace
  widths: Record<Panel, number>
  setWidths: React.Dispatch<React.SetStateAction<Record<Panel, number>>>
  inspectorMax: number
  zoom: number
}) {
  const maximum = (which: Panel) => (which === 'sidebar' ? limits.sidebar.max : inspectorMax)
  function change(which: Panel, width: number, persist = false) {
    const next = Math.max(limits[which].min, Math.min(maximum(which), width))
    setWidths((value) => ({ ...value, [which]: next }))
    if (persist)
      void w
        .savePreferences({ [which === 'sidebar' ? 'sidebarWidth' : 'inspectorWidth']: next })
        .catch(w.errorHandler)
    return next
  }
  function pointer(which: Panel, event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    event.currentTarget.setPointerCapture(event.pointerId)
    const start = event.clientX
    const initial = widths[which]
    let latest = initial
    const move = (move: PointerEvent) => {
      latest = change(which, initial + ((move.clientX - start) / zoom) * (which === 'sidebar' ? 1 : -1))
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      window.removeEventListener('pointercancel', stop)
      change(which, latest, true)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop)
    window.addEventListener('pointercancel', stop)
  }
  function keyboard(which: Panel, event: React.KeyboardEvent<HTMLDivElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const direction = (event.key === 'ArrowRight' ? 1 : -1) * (which === 'sidebar' ? 1 : -1)
    change(
      which,
      event.key === 'Home'
        ? limits[which].min
        : event.key === 'End'
          ? maximum(which)
          : widths[which] + direction * (event.shiftKey ? 50 : 10),
      true
    )
  }
  const separator = (which: Panel, label: string, value: number) => ({
    className: `resizer ${which === 'sidebar' ? 'left-resizer' : 'right-resizer'}`,
    role: 'separator',
    'aria-label': label,
    'aria-orientation': 'vertical' as const,
    'aria-valuemin': limits[which].min,
    'aria-valuemax': maximum(which),
    'aria-valuenow': value,
    tabIndex: 0,
    onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => pointer(which, event),
    onKeyDown: (event: React.KeyboardEvent<HTMLDivElement>) => keyboard(which, event)
  })
  return { separator }
}
