import koffi from 'koffi'
import type { WindowInfo } from '../../shared/computer'

let cached: ReturnType<typeof bind> | undefined

function bind() {
  const user = koffi.load('user32.dll')
  const dwm = koffi.load('dwmapi.dll')
  const RECT = koffi.struct('PRIME_RECT', {
    left: 'int32_t',
    top: 'int32_t',
    right: 'int32_t',
    bottom: 'int32_t'
  })
  const callback = koffi.proto('__stdcall', 'PRIME_WNDENUMPROC', 'int', ['intptr_t', 'intptr_t'])
  return {
    callback,
    enumWindows: user.func('__stdcall', 'EnumWindows', 'int', [
      koffi.pointer(callback),
      'intptr_t'
    ]),
    isVisible: user.func('__stdcall', 'IsWindowVisible', 'int', ['intptr_t']),
    isWindow: user.func('__stdcall', 'IsWindow', 'int', ['intptr_t']),
    owner: user.func('__stdcall', 'GetWindow', 'intptr_t', ['intptr_t', 'uint32_t']),
    titleLength: user.func('__stdcall', 'GetWindowTextLengthW', 'int', ['intptr_t']),
    title: user.func('__stdcall', 'GetWindowTextW', 'int', [
      'intptr_t',
      koffi.out(koffi.pointer('uint16_t')),
      'int'
    ]),
    processId: user.func('__stdcall', 'GetWindowThreadProcessId', 'uint32_t', [
      'intptr_t',
      koffi.out(koffi.pointer('uint32_t'))
    ]),
    windowRect: user.func('__stdcall', 'GetWindowRect', 'int', [
      'intptr_t',
      koffi.out(koffi.pointer(RECT))
    ]),
    cloaked: dwm.func('__stdcall', 'DwmGetWindowAttribute', 'int32_t', [
      'intptr_t',
      'uint32_t',
      koffi.out(koffi.pointer('int32_t')),
      'uint32_t'
    ]),
    frameBounds: dwm.func('__stdcall', 'DwmGetWindowAttribute', 'int32_t', [
      'intptr_t',
      'uint32_t',
      koffi.out(koffi.pointer(RECT)),
      'uint32_t'
    ])
  }
}

function api() {
  if (process.platform !== 'win32') throw new Error('Просмотр окон доступен только в Windows')
  cached ??= bind()
  return cached
}

function bounds(handle: number) {
  const rect = { left: 0, top: 0, right: 0, bottom: 0 }
  if (api().frameBounds(handle, 9, rect, 16) !== 0) api().windowRect(handle, rect)
  return {
    x: rect.left,
    y: rect.top,
    width: rect.right - rect.left,
    height: rect.bottom - rect.top
  }
}

function titleOf(handle: number) {
  const length = api().titleLength(handle)
  if (!length) return ''
  const buffer = new Uint16Array(length + 1)
  api().title(handle, buffer, length + 1)
  return String.fromCharCode(...buffer.subarray(0, length))
}

function describe(handle: number): WindowInfo {
  const pid = [0]
  api().processId(handle, pid)
  return { hwnd: String(handle), title: titleOf(handle), pid: pid[0], bounds: bounds(handle) }
}

export function listWindows(): WindowInfo[] {
  const native = api()
  const windows: WindowInfo[] = []
  const handler = koffi.register((handle: number) => {
    if (!native.isVisible(handle)) return 1
    const cloaked = [0]
    if (native.cloaked(handle, 14, cloaked, 4) === 0 && cloaked[0]) return 1
    const title = titleOf(handle)
    if (!title.trim() || title === 'Program Manager') return 1
    const info = describe(handle)
    if (info.bounds.width >= 80 && info.bounds.height >= 50) windows.push(info)
    return 1
  }, koffi.pointer(native.callback))
  try {
    if (!native.enumWindows(handler, 0))
      throw new Error('Windows could not enumerate desktop windows')
  } finally {
    koffi.unregister(handler)
  }
  return windows
}

export function windowInfo(hwnd: string): WindowInfo | undefined {
  const handle = Number(hwnd)
  if (!Number.isSafeInteger(handle) || !api().isWindow(handle)) return undefined
  return describe(handle)
}

export { claimWindowIdentity, verifyWindowIdentity } from './identity'
export { performWindowAction } from './input'
