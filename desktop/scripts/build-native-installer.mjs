import { spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const target = process.argv[2] || (process.platform === 'win32' ? 'nsis' : 'appimage,deb,rpm')
const library = process.platform === 'win32' ? 'prime_desktop_engine.dll' : process.platform === 'darwin' ? 'libprime_desktop_engine.dylib' : 'libprime_desktop_engine.so'
const publicKey = process.env.PRIME_UPDATER_PUBLIC_KEY?.trim()
const endpoint = process.env.PRIME_UPDATER_ENDPOINT?.trim()
const privateKey = process.env.TAURI_SIGNING_PRIVATE_KEY
const signed = Boolean(publicKey && endpoint && privateKey)
if ((publicKey || endpoint || privateKey) && !signed) {
  throw new Error('Signed builds require PRIME_UPDATER_PUBLIC_KEY, PRIME_UPDATER_ENDPOINT and TAURI_SIGNING_PRIVATE_KEY')
}
if (endpoint && new URL(endpoint).protocol !== 'https:') throw new Error('The updater endpoint must use HTTPS')
const directory = mkdtempSync(join(tmpdir(), 'prime-native-build-'))
try {
  const config = join(directory, 'updater.json')
  writeFileSync(config, JSON.stringify({
    bundle: {
      createUpdaterArtifacts: signed,
      resources: { [`engine/${library}`]: `engine/${library}`, 'engine/manifest.json': 'engine/manifest.json' },
      ...(process.platform === 'linux' ? {
        linux: {
          appimage: { bundleMediaFramework: true },
          deb: { depends: ['libwebkit2gtk-4.1-0', 'libgtk-3-0', 'libssl3', 'libayatana-appindicator3-1', 'gstreamer1.0-plugins-base', 'gstreamer1.0-plugins-good'] },
          rpm: { depends: ['libwebkit2gtk-4.1.so.0()(64bit)', 'libgtk-3.so.0()(64bit)', 'libssl.so.3()(64bit)', 'libcrypto.so.3()(64bit)', 'libayatana-appindicator3.so.1()(64bit)'] }
        }
      } : {})
    },
    ...(signed ? { plugins: { updater: { pubkey: publicKey, endpoints: [endpoint] } } } : {})
  }))
  const executable = join(root, 'node_modules', '@tauri-apps', 'cli', 'tauri.js')
  const result = spawnSync(process.execPath, [executable, 'build', '--bundles', target, '--config', config], {
    cwd: root,
    stdio: 'inherit',
    env: process.env
  })
  if (result.error) throw result.error
  process.exitCode = result.status ?? 1
} finally {
  rmSync(directory, { recursive: true, force: true })
}
