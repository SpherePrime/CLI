import { useEffect } from 'react'
import type { Workspace } from './useWorkspace'
import { matchesPointerShortcut, matchesShortcut } from '../../../shared/keybindings'
import { getShortcuts } from '../features/shortcuts/shortcutStore'

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
      const shortcuts = getShortcuts()
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      const run = (action: () => void) => {
        event.preventDefault()
        action()
      }
      if (matchesShortcut(event, shortcuts.commands))
        return run(() => {
          if (!dialogOpen) setModal('commands')
        })
      if (dialogOpen || event.defaultPrevented) return
      if (matchesShortcut(event, shortcuts.terminal) && w.project) return run(toggleTerminal)
      if (matchesShortcut(event, shortcuts.focus)) return run(toggleFocus)
      if (
        matchesShortcut(event, shortcuts.chatSearch) &&
        w.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      )
        return run(openSearch)
      if (matchesShortcut(event, shortcuts.settings)) return run(() => setModal('settings'))
      if (matchesShortcut(event, shortcuts.models))
        return run(() => window.dispatchEvent(new Event('prime:models')))
      if (matchesShortcut(event, shortcuts.newChat))
        return run(() => {
          if (w.project) void w.createSession()
        })
      if (matchesShortcut(event, shortcuts.zoomIn))
        return run(() => setZoom((value) => Math.min(2, value + 0.1)))
      if (matchesShortcut(event, shortcuts.zoomOut))
        return run(() => setZoom((value) => Math.max(0.8, value - 0.1)))
      if (matchesShortcut(event, shortcuts.zoomReset)) return run(() => setZoom(1))
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  })
  useEffect(() => {
    // Mouse bindings: back/forward side buttons (and any button with
    // modifiers) fire no keyboard events at all, so they are routed from
    // pointer events. A bare left click is not intercepted, it just runs
    // the bound action next to the click's own behavior.
    const pointerHandler = (event: PointerEvent) => {
      const shortcuts = getShortcuts()
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      const run = (action: () => void) => {
        if (event.button !== 0) event.preventDefault()
        action()
      }
      if (matchesPointerShortcut(event, shortcuts.commands))
        return run(() => {
          if (!dialogOpen) setModal('commands')
        })
      if (dialogOpen) return
      if (matchesPointerShortcut(event, shortcuts.terminal) && w.project) return run(toggleTerminal)
      if (matchesPointerShortcut(event, shortcuts.focus)) return run(toggleFocus)
      if (
        matchesPointerShortcut(event, shortcuts.chatSearch) &&
        w.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      )
        return run(openSearch)
      if (matchesPointerShortcut(event, shortcuts.settings)) return run(() => setModal('settings'))
      if (matchesPointerShortcut(event, shortcuts.models))
        return run(() => window.dispatchEvent(new Event('prime:models')))
      if (matchesPointerShortcut(event, shortcuts.newChat))
        return run(() => {
          if (w.project) void w.createSession()
        })
      if (matchesPointerShortcut(event, shortcuts.zoomIn))
        return run(() => setZoom((value) => Math.min(2, value + 0.1)))
      if (matchesPointerShortcut(event, shortcuts.zoomOut))
        return run(() => setZoom((value) => Math.max(0.8, value - 0.1)))
      if (matchesPointerShortcut(event, shortcuts.zoomReset)) return run(() => setZoom(1))
    }
    window.addEventListener('pointerdown', pointerHandler, true)
    return () => window.removeEventListener('pointerdown', pointerHandler, true)
  })
}
