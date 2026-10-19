import { randomUUID } from 'node:crypto'
import { posix } from 'node:path'
import type { RemoteProfile } from '../../shared/remote'
export function shellQuote(value: string) {
  return "'" + value.replaceAll("'", "'\\''") + "'"
}
export function remotePath(root: string, path: string) {
  if (
    path.includes('\\') ||
    path.includes('\0') ||
    posix.isAbsolute(path) ||
    path.split('/').includes('..')
  )
    throw new Error('remote.pathOutside')
  const result = posix.resolve(root, path || '.')
  if (result !== root && !result.startsWith(root.replace(/\/$/, '') + '/'))
    throw new Error('remote.pathOutside')
  return result
}
export function validateProfile(input: unknown): RemoteProfile {
  const value = input as Record<string, unknown>
  if (!value || typeof value !== 'object') throw new Error('remote.invalidProfile')
  const text = (key: string, limit = 1024) => {
    const item = value[key]
    if (typeof item !== 'string' || !item.trim() || item.length > limit || /[\0\r\n]/.test(item))
      throw new Error('remote.invalidProfile')
    return item.trim()
  }
  const id = value.id === undefined ? randomUUID() : text('id')
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id))
    throw new Error('remote.invalidProfile')
  const root = text('root')
  if (!root.startsWith('/') || root.includes('\\') || root.split('/').includes('..'))
    throw new Error('remote.invalidProfile')
  if (!Number.isInteger(value.port) || Number(value.port) < 1 || Number(value.port) > 65535)
    throw new Error('remote.invalidProfile')
  if (
    !['agent', 'key', 'password'].includes(String(value.auth)) ||
    !['bundled', 'existing'].includes(String(value.runtime))
  )
    throw new Error('remote.invalidProfile')
  const result: RemoteProfile = {
    id,
    name: text('name', 120),
    host: text('host', 253),
    port: Number(value.port),
    user: text('user', 120),
    root: posix.resolve(root),
    auth: value.auth as RemoteProfile['auth'],
    runtime: value.runtime as RemoteProfile['runtime']
  }
  if (result.auth === 'key') result.keyPath = text('keyPath', 4096)
  if (result.runtime === 'existing') {
    result.runtimePath = text('runtimePath', 4096)
    if (!result.runtimePath.startsWith('/')) throw new Error('remote.invalidProfile')
  }
  return result
}
