import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync, rmSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const tag = process.argv[2]
if (!/^v\d+\.\d+\.\d+$/.test(tag || '')) throw new Error('A stable release tag is required')
if (process.platform !== 'win32') throw new Error('The local desktop release builds the Windows package only')

const defaultEndpoint = 'https://github.com/SpherePrime/CLI/releases/latest/download/latest.json'
const keyFile = process.env.PRIME_UPDATER_KEY_FILE || join(homedir(), '.tauri', 'prime-updater.key')
const passwordFile = process.env.PRIME_UPDATER_PASSWORD_FILE || join(dirname(keyFile), 'prime-updater.password')

function readSecret(path) {
  return existsSync(path) ? readFileSync(path, 'utf8').trim() : undefined
}

const privateKey = process.env.TAURI_SIGNING_PRIVATE_KEY?.trim() || readSecret(keyFile)
if (!privateKey) {
  throw new Error(`Updater signing key not found. Put it in ${keyFile} or set TAURI_SIGNING_PRIVATE_KEY`)
}
const tauriConfig = JSON.parse(readFileSync(join(root, 'src-tauri', 'tauri.conf.json'), 'utf8'))
const publicKey = process.env.PRIME_UPDATER_PUBLIC_KEY?.trim() || tauriConfig.plugins?.updater?.pubkey
if (!publicKey) throw new Error('Updater public key is missing: set PRIME_UPDATER_PUBLIC_KEY or plugins.updater.pubkey')

const environment = {
  ...process.env,
  TAURI_SIGNING_PRIVATE_KEY: privateKey,
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD: process.env.TAURI_SIGNING_PRIVATE_KEY_PASSWORD ?? readSecret(passwordFile) ?? '',
  PRIME_UPDATER_PUBLIC_KEY: publicKey,
  PRIME_UPDATER_ENDPOINT: process.env.PRIME_UPDATER_ENDPOINT?.trim() || defaultEndpoint
}

function run(script, ...args) {
  const result = spawnSync(process.execPath, [join(root, 'scripts', script), ...args], { cwd: root, env: environment, stdio: 'inherit' })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

rmSync(join(root, 'src-tauri', 'target', 'release', 'bundle'), { recursive: true, force: true })
rmSync(join(root, 'release', 'native'), { recursive: true, force: true })
run('build-native-installer.mjs', 'nsis')
run('build-setup.mjs')
run('stage-native-release.mjs', tag)
