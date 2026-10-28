import { spawnSync } from 'node:child_process'
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { setupPayloads } from './setup-bundle.mjs'

const root = fileURLToPath(new URL('..', import.meta.url))
const release = join(root, 'src-tauri', 'target', 'release')
const bundle = join(release, 'bundle')
const version = JSON.parse(readFileSync(join(root, 'src-tauri', 'tauri.conf.json'), 'utf8')).version
function files(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    return entry.isDirectory() ? files(path) : [path]
  })
}

function size(path) {
  const info = statSync(path)
  return info.isDirectory()
    ? readdirSync(path).reduce((total, name) => total + size(join(path, name)), 0)
    : info.size
}
const windows = process.platform === 'win32'
const setupTarget = resolve(root, windows ? 'src-tauri' : 'installer/src-tauri', 'target')
const payloads = setupPayloads(process.platform, version, files(bundle))
const executable = (windows ? ['PrimeDesktop.exe', 'prime-native.exe'] : ['PrimeDesktop', 'prime-native'])
  .map((name) => join(release, name)).find(existsSync)
if (!executable) throw new Error('Release executable was not built')
const installedSize = size(executable) + size(join(release, 'engine'))

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`Setup build failed (${result.status ?? 1})`)
}

const directory = mkdtempSync(join(tmpdir(), 'prime-setup-build-'))
const tauri = join(root, 'node_modules', '@tauri-apps', 'cli', 'tauri.js')
try {
  let payload = payloads['app.exe']
  const args = [tauri, 'build']
  if (windows) {
    args.push('--no-bundle')
  } else {
    payload = join(directory, 'payloads')
    mkdirSync(payload)
    for (const [name, source] of Object.entries(payloads)) copyFileSync(source, join(payload, name))
    const config = join(directory, 'setup.json')
    writeFileSync(config, JSON.stringify({
      bundle: { active: true, createUpdaterArtifacts: false, linux: { appimage: { bundleMediaFramework: true } } }
    }))
    args.push('--bundles', 'appimage', '--config', config)
  }
  run(process.execPath, [join(root, 'node_modules', 'vite', 'bin', 'vite.js'), 'build', '--config', 'installer/vite.config.ts'])
  run(process.execPath, args, {
    cwd: join(root, 'installer'),
    env: {
      ...process.env,
      CARGO_TARGET_DIR: setupTarget,
      PRIME_SETUP_PAYLOAD: payload,
      PRIME_SETUP_INSTALLED_SIZE: String(installedSize)
    }
  })
  const output = join(bundle, 'setup')
  mkdirSync(output, { recursive: true })
  const target = join(output, `PrimeDesktop-Setup-${version}.${windows ? 'exe' : 'AppImage'}`)
  const source = windows ? join(release, 'prime-setup.exe')
    : join(setupTarget, 'release', 'bundle', 'appimage', `Prime Desktop Setup_${version}_amd64.AppImage`)
  copyFileSync(source, target)
  if (process.env.TAURI_SIGNING_PRIVATE_KEY || process.env.TAURI_SIGNING_PRIVATE_KEY_PATH) {
    run(process.execPath, [tauri, 'signer', 'sign', '--app-version', version, target])
  }
  console.log(target)
} finally {
  rmSync(directory, { recursive: true, force: true })
}
