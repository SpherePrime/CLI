import type { Terminal } from '@xterm/xterm'

export function bindTerminalClipboard(
  terminal: Terminal,
  element: HTMLElement,
  onError: (error: unknown) => void
) {
  const copySelection = () => {
    const selection = terminal.getSelection()
    if (!selection) return false
    void navigator.clipboard.writeText(selection).catch(onError)
    return true
  }
  const paste = () => {
    if (terminal.options.disableStdin) return
    void navigator.clipboard
      .readText()
      .then((text) => text && terminal.paste(text))
      .catch(onError)
  }
  terminal.attachCustomKeyEventHandler((event) => {
    if (event.type !== 'keydown' || !event.ctrlKey || event.altKey) return true
    const key = event.key.toLowerCase()
    if (key === 'c' && (event.shiftKey || terminal.hasSelection())) {
      event.preventDefault()
      if (copySelection()) terminal.clearSelection()
      return false
    }
    if (key === 'v' && event.shiftKey) {
      event.preventDefault()
      paste()
      return false
    }
    return true
  })
  const contextMenu = (event: MouseEvent) => {
    event.preventDefault()
    if (copySelection()) terminal.clearSelection()
    else paste()
  }
  element.addEventListener('contextmenu', contextMenu)
  return () => element.removeEventListener('contextmenu', contextMenu)
}
