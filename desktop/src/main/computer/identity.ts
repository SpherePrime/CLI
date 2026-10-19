import { randomInt } from 'node:crypto'
import koffi from 'koffi'
import { windowInfo } from './win32'
let properties:
  | {
      get: (hwnd: number, name: string) => number
      set: (hwnd: number, name: string, value: number) => number
    }
  | undefined
const propertyName = 'PrimeDesktop.AccessIdentity'
function api() {
  if (!properties) {
    const user = koffi.load('user32.dll')
    properties = {
      get: user.func('__stdcall', 'GetPropW', 'intptr_t', ['intptr_t', 'str16']),
      set: user.func('__stdcall', 'SetPropW', 'int', ['intptr_t', 'str16', 'intptr_t'])
    }
  }
  return properties
}
export function claimWindowIdentity(hwnd: string): string {
  const existing = api().get(Number(hwnd), propertyName)
  if (existing) return String(existing)
  const value = randomInt(1, 0x7fffffff)
  if (!api().set(Number(hwnd), propertyName, value))
    throw new Error('Cannot grant access to this window. It may run with elevated privileges.')
  return String(value)
}
export function verifyWindowIdentity(hwnd: string, pid: number, identity: string): boolean {
  return windowInfo(hwnd)?.pid === pid && String(api().get(Number(hwnd), propertyName)) === identity
}
