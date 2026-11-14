import { useEffect, useRef } from 'react'
import type { Workspace } from './useWorkspace'
import { matchesPointerShortcut, matchesShortcut } from '../../../shared/keybindings'
import { getShortcuts } from '../features/shortcuts/shortcutStore'

export function stepZoom(value: number, delta: number) {
  return Math.round(Math.max(0.8, Math.min(2, value + delta)) * 10) / 10
}

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
  const latest = useRef({
    workspace: w,
    setModal,
    toggleTerminal,
    toggleFocus,
    openSearch,
    setZoom
  })
  latest.current = {
    workspace: w,
    setModal,
    toggleTerminal,
    toggleFocus,
    openSearch,
    setZoom
  }

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const current = latest.current
      const shortcuts = getShortcuts()
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      const run = (action: () => void) => {
        event.preventDefault()
        action()
      }
      if (matchesShortcut(event, shortcuts.commands))
        return run(() => {
          if (!dialogOpen) current.setModal('commands')
        })
      if (dialogOpen || event.defaultPrevented) return
      if (matchesShortcut(event, shortcuts.terminal) && current.workspace.project)
        return run(current.toggleTerminal)
      if (matchesShortcut(event, shortcuts.focus)) return run(current.toggleFocus)
      if (
        matchesShortcut(event, shortcuts.chatSearch) &&
        current.workspace.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      )
        return run(current.openSearch)
      if (matchesShortcut(event, shortcuts.settings)) return run(() => current.setModal('settings'))
      if (matchesShortcut(event, shortcuts.models))
        return run(() => window.dispatchEvent(new Event('prime:models')))
      if (matchesShortcut(event, shortcuts.newChat))
        return run(() => {
          if (current.workspace.project) void current.workspace.createSession()
        })
      if (matchesShortcut(event, shortcuts.zoomIn))
        return run(() => current.setZoom((value) => stepZoom(value, 0.1)))
      if (matchesShortcut(event, shortcuts.zoomOut))
        return run(() => current.setZoom((value) => stepZoom(value, -0.1)))
      if (matchesShortcut(event, shortcuts.zoomReset)) return run(() => current.setZoom(1))
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  useEffect(() => {
    const pointerHandler = (event: PointerEvent) => {
      const current = latest.current
      const shortcuts = getShortcuts()
      const dialogOpen = Boolean(document.querySelector('dialog[open]'))
      const run = (action: () => void) => {
        if (event.button !== 0) event.preventDefault()
        action()
      }
      if (matchesPointerShortcut(event, shortcuts.commands))
        return run(() => {
          if (!dialogOpen) current.setModal('commands')
        })
      if (dialogOpen) return
      if (matchesPointerShortcut(event, shortcuts.terminal) && current.workspace.project)
        return run(current.toggleTerminal)
      if (matchesPointerShortcut(event, shortcuts.focus)) return run(current.toggleFocus)
      if (
        matchesPointerShortcut(event, shortcuts.chatSearch) &&
        current.workspace.messages.length &&
        !(event.target as HTMLElement).closest('.file-editor, .terminal-panel')
      )
        return run(current.openSearch)
      if (matchesPointerShortcut(event, shortcuts.settings))
        return run(() => current.setModal('settings'))
      if (matchesPointerShortcut(event, shortcuts.models))
        return run(() => window.dispatchEvent(new Event('prime:models')))
      if (matchesPointerShortcut(event, shortcuts.newChat))
        return run(() => {
          if (current.workspace.project) void current.workspace.createSession()
        })
      if (matchesPointerShortcut(event, shortcuts.zoomIn))
        return run(() => current.setZoom((value) => stepZoom(value, 0.1)))
      if (matchesPointerShortcut(event, shortcuts.zoomOut))
        return run(() => current.setZoom((value) => stepZoom(value, -0.1)))
      if (matchesPointerShortcut(event, shortcuts.zoomReset)) return run(() => current.setZoom(1))
    }
    window.addEventListener('pointerdown', pointerHandler, true)
    return () => window.removeEventListener('pointerdown', pointerHandler, true)
  }, [])
}
