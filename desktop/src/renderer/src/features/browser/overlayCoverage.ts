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
  '.prime-popover, .file-context-menu, .select-options, .context-menu, .toast-card'

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

export function isFullScreenOverlayOpen(root?: OverlayQueryRoot | null): boolean {
  const target = root ?? (typeof document !== 'undefined' ? (document as unknown as OverlayQueryRoot) : null)
  if (!target) return false
  return Boolean(target.querySelector(fullScreenOverlaySelector))
}

export function clipRectAgainstOverlays(
  rect: RectBounds,
  root?: OverlayQueryRoot | null
): RectBounds | null {
  const target = root ?? (typeof document !== 'undefined' ? (document as unknown as OverlayQueryRoot) : null)
  if (!target) return rect
  if (target.querySelector(fullScreenOverlaySelector)) return null

  let current = { ...rect }
  const elements = target.querySelectorAll(localizedOverlaySelector)
  for (let i = 0; i < elements.length; i++) {
    const el = elements[i]
    const box = el.getBoundingClientRect()
    if (!rectsOverlap(current, box)) continue

    if (box.top <= current.y && box.bottom > current.y) {
      const topCut = Math.round(box.bottom - current.y) + 4
      current.y += topCut
      current.height -= topCut
    } else if (box.bottom >= current.y + current.height && box.top < current.y + current.height) {
      const bottomCut = Math.round((current.y + current.height) - box.top) + 4
      current.height -= bottomCut
    } else if (box.left <= current.x && box.right > current.x) {
      const leftCut = Math.round(box.right - current.x) + 4
      current.x += leftCut
      current.width -= leftCut
    } else if (box.right >= current.x + current.width && box.left < current.x + current.width) {
      const rightCut = Math.round((current.x + current.width) - box.left) + 4
      current.width -= rightCut
    }
  }

  if (current.width < 100 || current.height < 100) {
    return null
  }
  return current
}

export function isOverlayCovering(
  rect: RectBounds,
  root?: OverlayQueryRoot | null
): boolean {
  return clipRectAgainstOverlays(rect, root) === null
}
