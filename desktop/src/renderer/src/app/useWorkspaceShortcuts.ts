import { useEffect } from 'react'
import type { Workspace } from './useWorkspace'

export function useWorkspaceShortcuts({
  workspace: w,
  setModal,
  toggleTerminal,
  toggleFocus,
  openSearch,
  setZoom
}: {
  workspace: Workspace
  setModal: (modal: string) => void
  toggleTerminal: () => void
  toggleFocus: () => void
  openSearch: () => void
  setZoom: React.Dispatch<React.SetStateAction<number>>
}) {
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      if (event.ctrlKey && event.code === 'KeyK') {
        event.preventDefault()
        if (!dialogOpen) setModal('commands')
        return
      }
      if (dialogOpen || event.defaultPrevented) return
      if (event.ctrlKey && event.code === 'Backquote' && w.project) {
        event.preventDefault()
        toggleTerminal()
      }
      if (event.ctrlKey && event.shiftKey && event.code === 'KeyF') {
        event.preventDefault()
        toggleFocus()
      }
      if (
        event.ctrlKey &&
        !event.shiftKey &&
        event.code === 'KeyF' &&
        w.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      ) {
        event.preventDefault()
        openSearch()
      }
      if (event.ctrlKey && event.code === 'Comma') {
        event.preventDefault()
        setModal('settings')
      }
      if (event.ctrlKey && event.code === 'KeyL') {
        event.preventDefault()
        window.dispatchEvent(new Event('prime:models'))
      }
      if (event.ctrlKey && event.code === 'KeyN') {
        event.preventDefault()
        if (w.project) void w.createSession()
      }
      if (event.ctrlKey && ['Equal', 'Minus', 'Digit0'].includes(event.code)) {
        event.preventDefault()
        setZoom((value) =>
          event.code === 'Digit0'
            ? 1
            : Math.max(0.8, Math.min(2, value + (event.code === 'Equal' ? 0.1 : -0.1)))
        )
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  })
}
