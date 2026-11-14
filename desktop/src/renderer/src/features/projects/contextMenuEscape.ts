export function handleContextMenuKeyDown(event: KeyboardEvent, close: () => void) {
  if (event.key !== 'Escape') return false
  event.preventDefault()
  event.stopPropagation()
  event.stopImmediatePropagation()
  close()
  return true
}
