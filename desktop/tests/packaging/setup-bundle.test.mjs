import { test } from 'node:test'
import assert from 'node:assert/strict'

test('Linux setup embeds one matching package for each installation method', async () => {
  const { setupPayloads } = await import('../../scripts/setup-bundle.mjs')
  assert.deepEqual(setupPayloads('linux', '1.2.3', [
    'appimage/Prime Desktop_1.2.2_amd64.AppImage',
    'appimage/Prime Desktop_1.2.3_amd64.AppImage',
    'deb/Prime Desktop_1.2.3_amd64.deb',
    'rpm/Prime Desktop-1.2.3-1.x86_64.rpm'
  ]), {
    'app.AppImage': 'appimage/Prime Desktop_1.2.3_amd64.AppImage',
    'app.deb': 'deb/Prime Desktop_1.2.3_amd64.deb',
    'app.rpm': 'rpm/Prime Desktop-1.2.3-1.x86_64.rpm'
  })
})

test('setup rejects incomplete Linux payloads', async () => {
  const { setupPayloads } = await import('../../scripts/setup-bundle.mjs')
  assert.throws(() => setupPayloads('linux', '1.2.3', [
    'appimage/Prime Desktop_1.2.3_amd64.AppImage'
  ]), /deb/)
})

test('Windows setup embeds NSIS rather than nesting another custom setup', async () => {
  const { setupPayloads } = await import('../../scripts/setup-bundle.mjs')
  assert.deepEqual(setupPayloads('win32', '1.2.3', [
    'nsis/Prime Desktop_1.2.3_x64-setup.exe',
    'setup/PrimeDesktop-Setup-1.2.3.exe'
  ]), { 'app.exe': 'nsis/Prime Desktop_1.2.3_x64-setup.exe' })
})

test('setup works when the repository parent directory contains Setup', async () => {
  const { setupPayloads } = await import('../../scripts/setup-bundle.mjs')
  assert.deepEqual(setupPayloads('win32', '1.2.3', [
    '/home/Setup/CLI/nsis/Prime Desktop_1.2.3_x64-setup.exe'
  ]), { 'app.exe': '/home/Setup/CLI/nsis/Prime Desktop_1.2.3_x64-setup.exe' })
})
