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
type MouseEventType = Pick<PointerEvent | MouseEvent, 'button' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'metaKey'>

const modifierCodes = /^(Control|Shift|Alt|Meta|OS)(Left|Right)?$/
// Mouse buttons, per the PointerEvent button codes: 0 primary (left),
// 1 auxiliary (middle), 2 secondary (right), 3 back, 4 forward, 5-6 extra.
// Keyboard event codes never start with "Mouse", so keyboard and mouse
// bindings live in one store without ever clashing. A bare mouse click (no
// modifiers) is a valid binding too: it does not preventDefault, it just
// runs the action next to what the click already does in the UI.
function mouseButton(button: number) {
  switch (button) {
    case 0:
      return 'MouseLeft'
    case 1:
      return 'MouseMiddle'
    case 2:
      return 'MouseRight'
    case 3:
      return 'MouseBack'
    case 4:
      return 'MouseForward'
    case 5:
      return 'MouseX1'
    case 6:
      return 'MouseX2'
    default:
      return null
  }
}
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

// Mouse buttons answer to pointer/mouse events, and the back/forward side
// buttons in particular are what keyboard binding UIs can never capture.
// Bare clicks bind too; no modifier is required.
export function mouseShortcutFromEvent(event: MouseEventType): string | null {
  const code = mouseButton(event.button)
  if (!code) return null
  const modifiers = [
    event.ctrlKey && 'Ctrl',
    event.altKey && 'Alt',
    event.shiftKey && 'Shift',
    event.metaKey && 'Meta'
  ].filter(Boolean)
  return [...modifiers, code].join('+')
}

// A stored shortcut is a mouse binding when its key part starts with "Mouse"
// (the modifiers prefix — "Ctrl+", "Alt+", etc. — may or may not be present).
function isMouseShortcut(shortcut: string) {
  return shortcut.split('+').some((part) => part.startsWith('Mouse'))
}

export function matchesPointerShortcut(event: MouseEventType, shortcut: string | undefined) {
  return !!shortcut && isMouseShortcut(shortcut) && mouseShortcutFromEvent(event) === shortcut
}

const mouseSymbols: Record<string, string> = {
  Left: 'LMB',
  Middle: 'MMB',
  Back: '⬅',
  Forward: '➡',
  X1: 'X1',
  X2: 'X2'
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
          : part.startsWith('Mouse')
            ? (mouseSymbols[part.slice(5)] ?? part)
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
