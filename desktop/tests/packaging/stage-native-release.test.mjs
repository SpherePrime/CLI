import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const script = fileURLToPath(new URL('../../scripts/stage-native-release.mjs', import.meta.url))

function stage(t, assets, platform) {
  const directory = mkdtempSync(join(tmpdir(), 'prime-stage-test-'))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  for (const [name, signature] of Object.entries(assets)) {
    const path = join(directory, 'src-tauri/target/release/bundle', name)
    mkdirSync(join(path, '..'), { recursive: true })
    writeFileSync(path, signature)
  }
  const result = spawnSync(process.execPath, [script, 'v1.2.3', platform], { cwd: directory, encoding: 'utf8' })
  return { result, directory }
}

test('Windows feed rejects an NSIS payload without the signed custom setup', (t) => {
  const { result } = stage(t, {
    'nsis/Prime Desktop_1.2.3_x64-setup.exe': 'payload',
    'nsis/Prime Desktop_1.2.3_x64-setup.exe.sig': 'nsis-signature'
  }, 'windows-x86_64')
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /custom setup/i)
})

test('Windows feed points to the custom setup and its signature', (t) => {
  const { result, directory } = stage(t, {
    'nsis/Prime Desktop_1.2.3_x64-setup.exe': 'payload',
    'nsis/Prime Desktop_1.2.3_x64-setup.exe.sig': 'nsis-signature',
    'setup/PrimeDesktop-Setup-1.2.3.exe': 'custom',
    'setup/PrimeDesktop-Setup-1.2.3.exe.sig': 'custom-signature'
  }, 'windows-x86_64')
  assert.equal(result.status, 0, result.stderr)
  const feed = JSON.parse(readFileSync(join(directory, 'release/native/windows-x86_64.json')))
  assert.equal(feed['windows-x86_64'].signature, 'custom-signature')
  assert.equal(feed['windows-x86_64'].url, 'https://github.com/SpherePrime/CLI/releases/download/v1.2.3/PrimeDesktop-Setup-1.2.3.exe')
})

test('Linux feed preserves package type for AppImage, Debian and RPM updates', (t) => {
  const { result, directory } = stage(t, {
    'appimage/Prime Desktop_1.2.3_amd64.AppImage': 'application',
    'appimage/Prime Desktop_1.2.3_amd64.AppImage.sig': 'appimage-signature',
    'deb/Prime Desktop_1.2.3_amd64.deb': 'debian',
    'deb/Prime Desktop_1.2.3_amd64.deb.sig': 'deb-signature',
    'rpm/Prime Desktop-1.2.3-1.x86_64.rpm': 'rpm',
    'rpm/Prime Desktop-1.2.3-1.x86_64.rpm.sig': 'rpm-signature',
    'setup/PrimeDesktop-Setup-1.2.3.AppImage': 'installer'
  }, 'linux-x86_64')
  assert.equal(result.status, 0, result.stderr)
  const feed = JSON.parse(readFileSync(join(directory, 'release/native/linux-x86_64.json')))
  assert.equal(feed['linux-x86_64-appimage'].signature, 'appimage-signature')
  assert.equal(feed['linux-x86_64-deb'].signature, 'deb-signature')
  assert.equal(feed['linux-x86_64-rpm'].signature, 'rpm-signature')
  assert.equal(feed['linux-x86_64'].url, feed['linux-x86_64-appimage'].url)
  assert.ok(!feed['linux-x86_64'].url.includes('Setup'))
})

test('stale artifacts cannot enter a new release feed', (t) => {
  const { result } = stage(t, {
    'setup/PrimeDesktop-Setup-1.2.2.exe': 'old',
    'setup/PrimeDesktop-Setup-1.2.2.exe.sig': 'old-signature'
  }, 'windows-x86_64')
  assert.notEqual(result.status, 0)
})

test('Linux release requires the custom setup alongside updater packages', (t) => {
  const { result } = stage(t, {
    'appimage/Prime Desktop_1.2.3_amd64.AppImage': 'application',
    'appimage/Prime Desktop_1.2.3_amd64.AppImage.sig': 'appimage-signature',
    'deb/Prime Desktop_1.2.3_amd64.deb': 'debian',
    'deb/Prime Desktop_1.2.3_amd64.deb.sig': 'deb-signature',
    'rpm/Prime Desktop-1.2.3-1.x86_64.rpm': 'rpm',
    'rpm/Prime Desktop-1.2.3-1.x86_64.rpm.sig': 'rpm-signature'
  }, 'linux-x86_64')
  assert.notEqual(result.status, 0)
  assert.match(result.stderr, /custom setup/i)
})
