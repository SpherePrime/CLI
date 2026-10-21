import { expect, it } from 'vitest'
import {
  defaultShortcuts,
  matchesShortcut,
  resolveShortcuts,
  shortcutConflicts,
  shortcutFromEvent,
  shortcutLabel
} from '../../src/shared/keybindings'

const key = (code: string, modifiers: Partial<Record<'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey', boolean>> = {}) => ({
  code,
  ctrlKey: false,
  altKey: false,
  shiftKey: false,
  metaKey: false,
  ...modifiers
})

it('reads shortcuts by physical key so they work in any layout', () => {
  expect(shortcutFromEvent(key('KeyK', { ctrlKey: true }))).toBe('Ctrl+KeyK')
  expect(shortcutFromEvent(key('KeyF', { ctrlKey: true, shiftKey: true }))).toBe('Ctrl+Shift+KeyF')
  expect(shortcutFromEvent(key('ControlLeft', { ctrlKey: true }))).toBeNull()
  expect(shortcutFromEvent(key('KeyA'))).toBeNull()
  expect(shortcutFromEvent(key('F5'))).toBe('F5')
  expect(matchesShortcut(key('KeyK', { ctrlKey: true }), defaultShortcuts.commands)).toBe(true)
  expect(matchesShortcut(key('KeyK', { ctrlKey: true }), '')).toBe(false)
  expect(shortcutLabel('Ctrl+Shift+Backquote')).toBe('Ctrl Shift `')
})

it('applies custom shortcuts and reports conflicts', () => {
  const shortcuts = resolveShortcuts({ models: 'Ctrl+KeyK', voice: '' })
  expect(shortcuts.models).toBe('Ctrl+KeyK')
  expect(shortcuts.voice).toBe('')
  expect(shortcutConflicts(shortcuts).get('commands')).toEqual(['models'])
  expect(shortcutConflicts(defaultShortcuts).size).toBe(0)
})
