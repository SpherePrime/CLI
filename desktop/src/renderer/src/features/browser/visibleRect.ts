export function visibleRect(element: HTMLElement) {
  const rect = element.getBoundingClientRect()
  let left = Math.max(0, rect.left)
  let top = Math.max(0, rect.top)
  let right = Math.min(window.innerWidth, rect.right)
  let bottom = Math.min(window.innerHeight, rect.bottom)
  for (let node = element.parentElement; node && node !== document.body; node = node.parentElement) {
    const style = getComputedStyle(node)
    if (style.display === 'none' || style.visibility === 'hidden') return null
    if (style.overflow === 'visible') continue
    const clip = node.getBoundingClientRect()
    left = Math.max(left, clip.left)
    top = Math.max(top, clip.top)
    right = Math.min(right, clip.right)
    bottom = Math.min(bottom, clip.bottom)
  }
  if (right - left < 2 || bottom - top < 2) return null
  return { x: left, y: top, width: right - left, height: bottom - top }
}
