import koffi from 'koffi'
import type { WindowAction } from '../../shared/computer'
import { windowInfo } from './win32'

let bindings: ReturnType<typeof bind> | undefined
function bind() {
  const user = koffi.load('user32.dll')
  return {
    foreground: user.func('__stdcall', 'GetForegroundWindow', 'intptr_t', []),
    focus: user.func('__stdcall', 'SetForegroundWindow', 'int', ['intptr_t']),
    bring: user.func('__stdcall', 'BringWindowToTop', 'int', ['intptr_t']),
    thread: user.func('__stdcall', 'GetWindowThreadProcessId', 'uint32_t', ['intptr_t', 'void *']),
    attach: user.func('__stdcall', 'AttachThreadInput', 'int', ['uint32_t', 'uint32_t', 'int']),
    currentThread: koffi
      .load('kernel32.dll')
      .func('__stdcall', 'GetCurrentThreadId', 'uint32_t', []),
    restore: user.func('__stdcall', 'ShowWindow', 'int', ['intptr_t', 'int']),
    minimized: user.func('__stdcall', 'IsIconic', 'int', ['intptr_t']),
    cursor: user.func('__stdcall', 'SetCursorPos', 'int', ['int', 'int']),
    atPoint: user.func('__stdcall', 'WindowFromPoint', 'intptr_t', ['int64_t']),
    ancestor: user.func('__stdcall', 'GetAncestor', 'intptr_t', ['intptr_t', 'uint32_t']),
    send: user.func('__stdcall', 'SendInput', 'uint32_t', ['uint32_t', 'void *', 'int'])
  }
}
function native() {
  if (process.platform !== 'win32' || process.arch !== 'x64')
    throw new Error('Window control requires Windows x64')
  return (bindings ??= bind())
}
function keyboard(vk: number, scan = 0, flags = 0) {
  const input = Buffer.alloc(40)
  input.writeUInt32LE(1, 0)
  input.writeUInt16LE(vk, 8)
  input.writeUInt16LE(scan, 10)
  input.writeUInt32LE(flags, 12)
  return input
}
function mouse(flags: number, delta = 0) {
  const input = Buffer.alloc(40)
  input.writeUInt32LE(delta >>> 0, 16)
  input.writeUInt32LE(flags, 20)
  return input
}
function send(inputs: Buffer[], guard: () => void) {
  guard()
  if (native().send(inputs.length, Buffer.concat(inputs), 40) !== inputs.length)
    throw new Error('Windows blocked input. The target may run with elevated privileges.')
}
const keys: Record<string, number> = {
  ENTER: 13,
  RETURN: 13,
  TAB: 9,
  ESC: 27,
  ESCAPE: 27,
  BACKSPACE: 8,
  DELETE: 46,
  SPACE: 32,
  HOME: 36,
  END: 35,
  PAGEUP: 33,
  PAGEDOWN: 34,
  LEFT: 37,
  UP: 38,
  RIGHT: 39,
  DOWN: 40,
  CTRL: 17,
  CONTROL: 17,
  SHIFT: 16,
  ALT: 18
}
function virtualKey(key: string) {
  if (keys[key]) return keys[key]
  if (/^[A-Z0-9]$/.test(key)) return key.charCodeAt(0)
  const functionKey = /^F([1-9]|1[0-2])$/.exec(key)
  if (functionKey) return 111 + Number(functionKey[1])
  throw new Error(`Unsupported key: ${key}`)
}
export function performWindowAction(hwnd: string, action: WindowAction, identityGuard: () => void) {
  const target = Number(hwnd)
  const api = native()
  identityGuard()
  if (action.action === 'focus') {
    if (api.minimized(target)) api.restore(target, 9)
    const current = api.currentThread()
    const foreground = api.thread(api.foreground(), null)
    const attached = foreground !== current && !!api.attach(current, foreground, 1)
    try {
      api.bring(target)
      api.focus(target)
    } finally {
      if (attached) api.attach(current, foreground, 0)
    }
    if (api.foreground() !== target)
      throw new Error(
        'Windows did not allow focusing the selected window. Focus it manually and retry.'
      )
    return
  }
  const guard = () => {
    identityGuard()
    if (api.foreground() !== target)
      throw new Error(
        'The selected window is not in the foreground. Use focus explicitly or ask the user to activate it.'
      )
  }
  guard()
  if (action.action === 'type') {
    const inputs: Buffer[] = []
    for (let index = 0; index < action.text.length; index++) {
      const code = action.text.charCodeAt(index)
      inputs.push(keyboard(0, code, 4), keyboard(0, code, 6))
    }
    if (inputs.length) send(inputs, guard)
    return
  }
  if (action.action === 'press_key') {
    const tokens = action.key
      .toUpperCase()
      .split('+')
      .map((value) => value.trim())
    if (tokens.length > 4 || tokens.some((value) => !value))
      throw new Error('Invalid key combination')
    if (
      (tokens.includes('ALT') &&
        (tokens.includes('TAB') || tokens.includes('ESC') || tokens.includes('ESCAPE'))) ||
      ((tokens.includes('CTRL') || tokens.includes('CONTROL')) &&
        (tokens.includes('ESC') ||
          tokens.includes('ESCAPE') ||
          (tokens.includes('ALT') && tokens.includes('DELETE'))))
    )
      throw new Error('Desktop-wide shortcuts are not allowed for window control')
    const codes = tokens.map(virtualKey)
    send(
      [
        ...codes.map((code) => keyboard(code)),
        ...[...codes].reverse().map((code) => keyboard(code, 0, 2))
      ],
      guard
    )
    return
  }
  const info = windowInfo(hwnd)
  if (
    !info ||
    action.x < 0 ||
    action.y < 0 ||
    action.x >= info.bounds.width ||
    action.y >= info.bounds.height
  )
    throw new Error('Coordinates are outside the selected window')
  const x = Math.round(info.bounds.x + action.x)
  const y = Math.round(info.bounds.y + action.y)
  const point = BigInt.asIntN(64, (BigInt(y >>> 0) << 32n) | BigInt(x >>> 0))
  if (api.ancestor(api.atPoint(point), 2) !== target)
    throw new Error('The selected point is covered by another window')
  guard()
  if (!api.cursor(x, y)) throw new Error('Could not move the pointer')
  if (action.action === 'scroll') send([mouse(0x0800, action.delta)], guard)
  else {
    const flags = action.button === 'right' ? [8, 16] : [2, 4]
    const inputs = Array.from({ length: action.count ?? 1 }, () =>
      flags.map((flag) => mouse(flag))
    ).flat()
    send(inputs, guard)
  }
}
