import { useSyncExternalStore } from 'react'
import { defaultShortcuts, resolveShortcuts, type Shortcuts } from '../../../../shared/keybindings'

let current: Shortcuts = defaultShortcuts
const listeners = new Set<() => void>()

export function applyShortcuts(custom?: Partial<Record<string, string>>) {
  const next = resolveShortcuts(custom)
  if (Object.keys(next).every((key) => next[key as keyof Shortcuts] === current[key as keyof Shortcuts]))
    return
  current = next
  for (const listener of listeners) listener()
}

export function getShortcuts() {
  return current
}

export function useShortcuts() {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    getShortcuts
  )
}
