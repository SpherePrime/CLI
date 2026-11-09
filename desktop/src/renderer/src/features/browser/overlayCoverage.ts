export interface RectBounds {
  x: number
  y: number
  width: number
  height: number
}

export interface BoxBounds {
  left: number
  top: number
  right: number
  bottom: number
  width: number
  height: number
}

export interface OverlayElement {
  getBoundingClientRect(): BoxBounds
}

export interface OverlayQueryRoot {
  querySelector(selector: string): unknown | null
  querySelectorAll(selector: string): ArrayLike<OverlayElement>
}

const fullScreenOverlaySelector =
  'dialog[open], .update-screen, .startup-screen:not(.hidden)'

const localizedOverlaySelector =
  '.prime-popover, .file-context-menu, .select-options'

export function rectsOverlap(rect: RectBounds, box: BoxBounds): boolean {
  if (box.width <= 0 || box.height <= 0) {
    return false
  }
  return !(
    box.right <= rect.x ||
    box.left >= rect.x + rect.width ||
    box.bottom <= rect.y ||
    box.top >= rect.y + rect.height
  )
}

export function isOverlayCovering(
  rect: RectBounds,
  root?: OverlayQueryRoot | null
): boolean {
  const target = root ?? (typeof document !== 'undefined' ? (document as unknown as OverlayQueryRoot) : null)
  if (!target) {
    return false
  }
  if (target.querySelector(fullScreenOverlaySelector)) {
    return true
  }
  const elements = target.querySelectorAll(localizedOverlaySelector)
  for (let i = 0; i < elements.length; i++) {
    const el = elements[i]
    const box = el.getBoundingClientRect()
    if (rectsOverlap(rect, box)) {
      return true
    }
  }
  return false
}
