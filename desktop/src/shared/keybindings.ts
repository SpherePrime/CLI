export const shortcutActions = [
  'commands',
  'chatSearch',
  'focus',
  'settings',
  'newChat',
  'models',
  'terminal',
  'voice',
  'saveFile',
  'zoomIn',
  'zoomOut',
  'zoomReset'
] as const

export type ShortcutAction = (typeof shortcutActions)[number]
export type Shortcuts = Record<ShortcutAction, string>

export const defaultShortcuts: Shortcuts = {
  commands: 'Ctrl+KeyK',
  chatSearch: 'Ctrl+KeyF',
  focus: 'Ctrl+Shift+KeyF',
  settings: 'Ctrl+Comma',
  newChat: 'Ctrl+KeyN',
  models: 'Ctrl+KeyL',
  terminal: 'Ctrl+Backquote',
  voice: 'Alt+KeyV',
  saveFile: 'Ctrl+KeyS',
  zoomIn: 'Ctrl+Equal',
  zoomOut: 'Ctrl+Minus',
  zoomReset: 'Ctrl+Digit0'
}

type KeyEvent = Pick<KeyboardEvent, 'code' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'>

const modifierCodes = /^(Control|Shift|Alt|Meta|OS)(Left|Right)?$/
const symbols: Record<string, string> = {
  Backquote: '`',
  Minus: '-',
  Equal: '=',
  BracketLeft: '[',
  BracketRight: ']',
  Backslash: '\\',
  Semicolon: ';',
  Quote: "'",
  Comma: ',',
  Period: '.',
  Slash: '/',
  Space: 'Space',
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→'
}

export function shortcutFromEvent(event: KeyEvent): string | null {
  if (!event.code || modifierCodes.test(event.code)) return null
  const modifiers = [
    event.ctrlKey && 'Ctrl',
    event.altKey && 'Alt',
    event.shiftKey && 'Shift',
    event.metaKey && 'Meta'
  ].filter(Boolean)
  const functionKey = /^F([1-9]|1[0-9]|2[0-4])$/.test(event.code)
  if (!modifiers.length && !functionKey) return null
  return [...modifiers, event.code].join('+')
}

export function matchesShortcut(event: KeyEvent, shortcut: string | undefined) {
  return Boolean(shortcut) && shortcutFromEvent(event) === shortcut
}

export function shortcutLabel(shortcut: string | undefined) {
  if (!shortcut) return ''
  return shortcut
    .split('+')
    .map((part) =>
      part.startsWith('Key')
        ? part.slice(3)
        : part.startsWith('Digit')
          ? part.slice(5)
          : (symbols[part] ?? part)
    )
    .join(' ')
}

export function resolveShortcuts(custom?: Partial<Record<string, string>>): Shortcuts {
  const result = { ...defaultShortcuts }
  for (const action of shortcutActions) {
    const value = custom?.[action]
    if (typeof value === 'string') result[action] = value
  }
  return result
}

export function shortcutConflicts(shortcuts: Shortcuts) {
  const owners = new Map<string, ShortcutAction[]>()
  for (const action of shortcutActions) {
    const value = shortcuts[action]
    if (value) owners.set(value, [...(owners.get(value) ?? []), action])
  }
  const conflicts = new Map<ShortcutAction, ShortcutAction[]>()
  for (const actions of owners.values())
    if (actions.length > 1)
      for (const action of actions)
        conflicts.set(
          action,
          actions.filter((other) => other !== action)
        )
  return conflicts
}
