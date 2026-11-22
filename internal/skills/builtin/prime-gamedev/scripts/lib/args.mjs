import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

export function parseFlagArgs(argv) {
  const flags = {}
  const positional = []

  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i]
    if (!token.startsWith('--')) {
      positional.push(token)
      continue
    }

    const eqIndex = token.indexOf('=')
    if (eqIndex !== -1) {
      const key = token.slice(2, eqIndex)
      const value = token.slice(eqIndex + 1)
      flags[key] = value
      continue
    }

    const key = token.slice(2)
    const next = argv[i + 1]
    if (next !== undefined && !next.startsWith('--')) {
      flags[key] = next
      i += 1
    } else {
      flags[key] = true
    }
  }

  return { flags, positional }
}

export function parsePositiveNumber(rawValue, fallbackValue) {
  if (rawValue === undefined || rawValue === true || rawValue === '') {
    return fallbackValue
  }
  const parsed = Number(rawValue)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallbackValue
  }
  return parsed
}

export function parsePositiveInteger(rawValue, fallbackValue) {
  const num = parsePositiveNumber(rawValue, fallbackValue)
  return Math.max(1, Math.floor(num))
}

export function resolveOutputDirectory(rawOut, prefix) {
  if (typeof rawOut === 'string' && rawOut.trim() !== '') {
    return resolve(rawOut.trim())
  }
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  return join(tmpdir(), `${prefix}-${stamp}`)
}
