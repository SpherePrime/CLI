import { expect, it } from 'vitest'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { configureDevelopmentProfile } from '../../src/main/developmentProfile'

function profileApp(root: string, packaged = false, explicitProfile = false) {
  const paths = { userData: root, sessionData: root }
  return {
    isPackaged: packaged,
    commandLine: { hasSwitch: (name: string) => name === 'user-data-dir' && explicitProfile },
    getPath: (name: keyof typeof paths) => paths[name],
    setPath: (name: keyof typeof paths, value: string) => { paths[name] = value }
  }
}

it('isolates development Chromium storage and settings from the installed profile', () => {
  const root = mkdtempSync(join(tmpdir(), 'prime-profile-'))
  const preferences = join(root, 'preferences.json')
  writeFileSync(preferences, '{"theme":"dark"}')
  const app = profileApp(root)
  configureDevelopmentProfile(app)
  expect(app.getPath('userData')).toBe(root + '-dev')
  expect(app.getPath('sessionData')).toBe(root + '-dev')
  writeFileSync(join(app.getPath('userData'), 'preferences.json'), '{"theme":"light"}')
  expect(readFileSync(preferences, 'utf8')).toBe('{"theme":"dark"}')
})

it.each([[true, false], [false, true]])(
  'preserves packaged profiles and explicit development profiles (%s, %s)',
  (packaged, explicitProfile) => {
    const root = mkdtempSync(join(tmpdir(), 'prime-profile-'))
    const app = profileApp(root, packaged, explicitProfile)
    configureDevelopmentProfile(app)
    expect(app.getPath('userData')).toBe(root)
    expect(app.getPath('sessionData')).toBe(root)
  }
)
